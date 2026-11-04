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
	monitor := &Monitor{}
	notifyTicker := time.NewTicker(30 * time.Second)
	eventTicker := time.NewTicker(60 * time.Second)
	defer notifyTicker.Stop()
	defer eventTicker.Stop()
	for {
		poll(ctx, p, seen, &first, log)
		monitorEvents(ctx, p, monitor, log)
		select {
		case <-ctx.Done():
			return
		case <-notifyTicker.C:
		case <-eventTicker.C:
		}
	}
}

func monitorEvents(ctx context.Context, p provider.Provider, monitor *Monitor, log *slog.Logger) {
	settings, err := p.Settings(ctx)
	if err != nil || (settings.DiscordWebhook == "" && settings.NtfyURL == "" && settings.WebhookURL == "") {
		return
	}
	eventCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	snapshot, err := p.Snapshot(eventCtx)
	if err != nil {
		return
	}
	for _, message := range monitor.Observe(snapshot) {
		if !Allowed(settings, message) {
			continue
		}
		if err := Send(eventCtx, settings, message); err != nil {
			log.Warn("send event alert", "title", message.Title, "error", err)
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
		message := FromModel(notification)
		if !Allowed(settings, message) {
			continue
		}
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		if err := Send(ctx, settings, message); err != nil {
			log.Warn("forward notification", "id", notification.ID, "error", err)
		}
		cancel()
	}
	*first = false
}
