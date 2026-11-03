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
