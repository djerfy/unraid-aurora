// Package notify forwards Aurora notifications to user-configured webhooks.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurora/internal/model"
)

type Message struct {
	Title       string
	Subject     string
	Description string
	Importance  string
	Source      string
	Timestamp   time.Time
}

func FromModel(n model.Notification) Message {
	title := n.Title
	if title == "" {
		title = n.Subject
	}
	return Message{
		Title: title, Subject: n.Subject, Description: n.Description,
		Importance: n.Importance, Source: n.Source, Timestamp: n.Timestamp,
	}
}

// Send delivers a message to every configured channel and reports the first
// failure while still attempting the other channels.
func Send(ctx context.Context, settings model.Settings, message Message) error {
	var errs []string
	if settings.DiscordWebhook != "" {
		if err := sendDiscord(ctx, settings.DiscordWebhook, message); err != nil {
			errs = append(errs, "discord: "+err.Error())
		}
	}
	if settings.NtfyURL != "" {
		if err := sendNtfy(ctx, settings.NtfyURL, message); err != nil {
			errs = append(errs, "ntfy: "+err.Error())
		}
	}
	if settings.WebhookURL != "" {
		if err := sendWebhook(ctx, settings.WebhookURL, message); err != nil {
			errs = append(errs, "webhook: "+err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func SendTest(ctx context.Context, settings model.Settings) error {
	if settings.DiscordWebhook == "" && settings.NtfyURL == "" && settings.WebhookURL == "" {
		return errors.New("no notification channel is configured")
	}
	return Send(ctx, settings, Message{
		Title: "Aurora notification test", Subject: "Test", Importance: "normal",
		Description: "This is a test notification from Aurora.", Timestamp: time.Now(),
	})
}

func sendDiscord(ctx context.Context, endpoint string, message Message) error {
	content := formatMessage(message)
	if len(content) > 1900 {
		content = content[:1900]
	}
	return postJSON(ctx, endpoint, map[string]string{"content": content})
}

func sendWebhook(ctx context.Context, endpoint string, message Message) error {
	return postJSON(ctx, endpoint, map[string]any{
		"title": message.Title, "subject": message.Subject, "description": message.Description,
		"importance": message.Importance, "source": message.Source,
		"timestamp": message.Timestamp.UTC().Format(time.RFC3339),
	})
}

func sendNtfy(ctx context.Context, endpoint string, message Message) error {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errors.New("invalid ntfy URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, parsed.String(), strings.NewReader(formatMessage(message)))
	if err != nil {
		return err
	}
	req.Header.Set("Title", message.Title)
	req.Header.Set("Priority", ntfyPriority(message.Importance))
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	return doRequest(req)
}

func postJSON(ctx context.Context, endpoint string, payload any) error {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errors.New("invalid webhook URL")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, parsed.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return doRequest(req)
}

func doRequest(req *http.Request) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("channel returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func formatMessage(message Message) string {
	title := strings.TrimSpace(message.Title)
	if title == "" {
		title = "Aurora"
	}
	text := title
	if message.Subject != "" && message.Subject != title {
		text += "\n" + message.Subject
	}
	if message.Description != "" {
		text += "\n" + message.Description
	}
	if message.Importance != "" {
		text += "\n[" + message.Importance + "]"
	}
	return text
}

func ntfyPriority(importance string) string {
	switch strings.ToLower(importance) {
	case "alert", "critical", "error":
		return "urgent"
	case "warning", "warn":
		return "high"
	default:
		return "default"
	}
}
