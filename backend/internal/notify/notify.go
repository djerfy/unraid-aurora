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
	"aurora/internal/outbound"
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
		if err := sendWebhook(ctx, settings, message); err != nil {
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

func sendWebhook(ctx context.Context, settings model.Settings, message Message) error {
	endpoint := strings.TrimSpace(settings.WebhookURL)
	parsed, err := url.Parse(endpoint)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errors.New("invalid webhook URL")
	}
	method := strings.ToUpper(strings.TrimSpace(settings.WebhookMethod))
	switch method {
	case "", "POST":
		method = http.MethodPost
	case http.MethodGet, http.MethodPut, http.MethodDelete:
	default:
		return errors.New("webhook method must be GET, POST, PUT or DELETE")
	}
	body := settings.WebhookBody
	if strings.TrimSpace(body) == "" {
		body = fmt.Sprintf(`{"title":%q,"subject":%q,"description":%q,"importance":%q,"source":%q,"timestamp":%q}`,
			message.Title, message.Subject, message.Description, message.Importance, message.Source,
			message.Timestamp.UTC().Format(time.RFC3339))
	} else {
		body = replaceWebhookPlaceholders(body, message)
	}
	var reader io.Reader
	if method != http.MethodGet || strings.TrimSpace(settings.WebhookBody) != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, parsed.String(), reader)
	if err != nil {
		return err
	}
	if reader != nil {
		contentType := "application/json"
		if settings.WebhookHeaders != nil && settings.WebhookHeaders["Content-Type"] != "" {
			contentType = settings.WebhookHeaders["Content-Type"]
		} else if !strings.HasPrefix(strings.TrimSpace(body), "{") && !strings.HasPrefix(strings.TrimSpace(body), "[") {
			contentType = "text/plain; charset=utf-8"
		}
		req.Header.Set("Content-Type", contentType)
	}
	for key, value := range settings.WebhookHeaders {
		if strings.TrimSpace(key) == "" || strings.EqualFold(key, "Content-Type") {
			continue
		}
		req.Header.Set(key, replaceWebhookPlaceholders(value, message))
	}
	return doRequest(req)
}

func replaceWebhookPlaceholders(template string, message Message) string {
	replacements := map[string]string{
		"{title}": message.Title, "{subject}": message.Subject, "{description}": message.Description,
		"{importance}": message.Importance, "{source}": message.Source,
		"{timestamp}": message.Timestamp.UTC().Format(time.RFC3339),
	}
	out := template
	for key, value := range replacements {
		out = strings.ReplaceAll(out, key, value)
	}
	return out
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
	client := outbound.Client(10 * time.Second)
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

var importanceRank = map[string]int{
	"info": 0, "normal": 1, "warning": 2, "alert": 3, "critical": 4, "error": 4,
}

// Allowed applies the user's notification filters before a message is sent.
// Unknown importance values are treated as normal so a new Unraid level is not
// silently dropped.
func Allowed(settings model.Settings, message Message) bool {
	minimum := importanceRank[strings.ToLower(strings.TrimSpace(settings.NotifyMinImportance))]
	rank, ok := importanceRank[strings.ToLower(strings.TrimSpace(message.Importance))]
	if !ok {
		rank = importanceRank["normal"]
	}
	if rank < minimum {
		return false
	}
	for _, source := range settings.NotifyMutedSources {
		if strings.EqualFold(strings.TrimSpace(source), strings.TrimSpace(message.Source)) {
			return false
		}
	}
	if enabled, ok := settings.NotifyEvents[NotificationCategory(message)]; ok && !enabled {
		return false
	}
	return true
}

var eventCategories = []struct {
	name     string
	keywords []string
}{
	{"mover", []string{"mover", "move"}},
	{"array", []string{"array", "parity", "sync", "rebuild", "check"}},
	{"disk", []string{"disk", "smart", "temperature", "unmount", "mount"}},
	{"docker", []string{"docker", "container"}},
	{"vm", []string{"vm", "virtual machine", "libvirt", "kvm"}},
	{"ups", []string{"ups", "battery", "power"}},
	{"plugin", []string{"plugin", "update", "package"}},
}

// NotificationCategory classifies an Unraid notification for the per-event
// filters. Unknown messages fall back to "system" so they can be muted too.
func NotificationCategory(message Message) string {
	haystack := strings.ToLower(strings.Join([]string{message.Source, message.Title, message.Subject, message.Description}, " "))
	for _, category := range eventCategories {
		for _, keyword := range category.keywords {
			if strings.Contains(haystack, keyword) {
				return category.name
			}
		}
	}
	return "system"
}
