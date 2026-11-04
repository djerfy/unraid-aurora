package redact

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONRedactsSensitiveKeys(t *testing.T) {
	raw := []byte(`{"discordWebhook":"https://discord.test/secret","theme":"dark","nested":{"apiToken":"abc","safe":"yes"}}`)
	out := JSON(raw)
	var value map[string]any
	if err := json.Unmarshal(out, &value); err != nil {
		t.Fatal(err)
	}
	if value["discordWebhook"] != placeholder {
		t.Fatalf("webhook was not redacted: %v", value)
	}
	if value["theme"] != "dark" {
		t.Fatalf("unrelated value changed: %v", value)
	}
	nested := value["nested"].(map[string]any)
	if nested["apiToken"] != placeholder || nested["safe"] != "yes" {
		t.Fatalf("nested values were not handled: %v", nested)
	}
}

func TestConfigRedactsSensitiveKeysAndKeepsComments(t *testing.T) {
	raw := "---\n# comment\ncsrf_token=\"abc\"\nshareComment=\"media\"\r\n"
	out := string(Config([]byte(raw)))
	if !strings.Contains(out, `csrf_token="`+placeholder+`"`) {
		t.Fatalf("token was not redacted: %q", out)
	}
	if !strings.Contains(out, `shareComment="media"`) {
		t.Fatalf("unrelated value changed: %q", out)
	}
	if !strings.Contains(out, "# comment") || !strings.Contains(out, "\r\n") {
		t.Fatalf("comments or line endings were lost: %q", out)
	}
}

func TestInvalidJSONIsPreserved(t *testing.T) {
	raw := []byte(`{not json`)
	if string(JSON(raw)) != string(raw) {
		t.Fatal("invalid JSON should be preserved")
	}
}
