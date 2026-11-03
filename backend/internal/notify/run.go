package notify

import (
	"context"
	"log/slog"
	"time"

	"aurora/internal/provider"
)

// Run forwards Unraid notifications that appear after Aurora starts. Existing
// notifications are marked as seen on the first pass to avoid a startup flood.
func Run(ctx context.Context, p provider.Provider, log *slog.Logger) {
	seen := map[string]bool{}
	first := true
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		poll(ctx, p, seen, &first, log)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func poll(ctx context.Context, p provider.Provider, seen map[string]bool, first *bool, log *slog.Logger) {
	settings, err := p.Settings(ctx)
	if err != nil || (settings.DiscordWebhook == "" && settings.NtfyURL == "" && settings.WebhookURL == "") {
		*first = false
		return
	}
	notifications, err := p.Notifications(ctx)
	if err != nil {
		return
	}
	for _, notification := range notifications {
		if notification.Read || notification.Archived || seen[notification.ID] {
			continue
		}
		seen[notification.ID] = true
		if *first {
			continue
		}
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		if err := Send(ctx, settings, FromModel(notification)); err != nil {
			log.Warn("forward notification", "id", notification.ID, "error", err)
		}
		cancel()
	}
	*first = false
}
