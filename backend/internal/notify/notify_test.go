package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurora/internal/model"
)

func TestSendDiscordAndWebhook(t *testing.T) {
	var discord, webhook map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch r.URL.Path {
		case "/discord":
			_ = json.Unmarshal(body, &discord)
		case "/webhook":
			_ = json.Unmarshal(body, &webhook)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	err := Send(context.Background(), model.Settings{
		DiscordWebhook: server.URL + "/discord",
		WebhookURL:     server.URL + "/webhook",
	}, Message{Title: "Disk warning", Description: "disk1 is hot", Importance: "warning"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(discord["content"].(string), "disk1 is hot") {
		t.Fatalf("unexpected Discord payload: %+v", discord)
	}
	if webhook["title"] != "Disk warning" || webhook["importance"] != "warning" {
		t.Fatalf("unexpected webhook payload: %+v", webhook)
	}
}

func TestSendNtfy(t *testing.T) {
	var title, priority, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		title = r.Header.Get("Title")
		priority = r.Header.Get("Priority")
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := Send(context.Background(), model.Settings{NtfyURL: server.URL + "/aurora"}, Message{Title: "SMART", Importance: "critical"}); err != nil {
		t.Fatal(err)
	}
	if title != "SMART" || priority != "urgent" || !strings.Contains(body, "SMART") {
		t.Fatalf("unexpected ntfy request: title=%q priority=%q body=%q", title, priority, body)
	}
}

func TestSendRejectsInvalidURL(t *testing.T) {
	if err := Send(context.Background(), model.Settings{WebhookURL: "file:///etc/passwd"}, Message{}); err == nil {
		t.Fatal("invalid scheme was accepted")
	}
}

func TestSendCustomWebhookMethodHeadersAndTemplate(t *testing.T) {
	var method, authorization, source, contentType, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		authorization = r.Header.Get("Authorization")
		source = r.Header.Get("X-Source")
		contentType = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := Send(context.Background(), model.Settings{
		WebhookURL:     server.URL + "/hook",
		WebhookMethod:  "PUT",
		WebhookHeaders: map[string]string{"Authorization": "Bearer secret", "X-Source": "{source}"},
		WebhookBody:    `{"title":"{title}","description":"{description}"}`,
	}, Message{Title: "Array alert", Description: "parity failed", Source: "array", Importance: "alert"})
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPut || authorization != "Bearer secret" || source != "array" {
		t.Fatalf("unexpected request: method=%q auth=%q source=%q", method, authorization, source)
	}
	if !strings.Contains(contentType, "application/json") {
		t.Fatalf("unexpected content type %q", contentType)
	}
	if !strings.Contains(body, `"title":"Array alert"`) || !strings.Contains(body, `"description":"parity failed"`) {
		t.Fatalf("unexpected body %q", body)
	}

	err = Send(context.Background(), model.Settings{
		WebhookURL: server.URL + "/hook", WebhookMethod: "PATCH",
	}, Message{Title: "x"})
	if err == nil {
		t.Fatal("unsupported method was accepted")
	}
}

func TestAllowedFilters(t *testing.T) {
	settings := model.Settings{NotifyMinImportance: "warning", NotifyMutedSources: []string{"docker"}}
	if Allowed(settings, Message{Importance: "normal"}) {
		t.Fatal("message below the minimum importance was allowed")
	}
	if !Allowed(settings, Message{Importance: "alert", Source: "array"}) {
		t.Fatal("message at the minimum importance was blocked")
	}
	if Allowed(settings, Message{Importance: "alert", Source: "Docker"}) {
		t.Fatal("muted source was not blocked")
	}
	if !Allowed(model.Settings{}, Message{Importance: "unknown"}) {
		t.Fatal("unknown importance should degrade to normal")
	}
}
