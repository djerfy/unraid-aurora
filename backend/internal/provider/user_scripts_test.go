package provider

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func userScriptTestProvider(t *testing.T) *Live {
	t.Helper()
	boot := t.TempDir()
	dir := filepath.Join(boot, "config", "plugins", "user.scripts", "scripts", "backup")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "script"), []byte("#!/bin/bash\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "name"), []byte("Nightly backup\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(boot, "config", "plugins", "user.scripts")
	schedules := map[string]any{
		filepath.Join(dir, "script"): map[string]any{
			"script": filepath.Join(dir, "script"), "frequency": "custom",
			"id": "backup", "custom": "0 3 * * *",
		},
	}
	raw, _ := json.Marshal(schedules)
	if err := os.WriteFile(filepath.Join(configDir, "schedule.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return &Live{bootDir: boot, stateDir: t.TempDir()}
}

func TestUserScriptsReadsPluginSchedule(t *testing.T) {
	live := userScriptTestProvider(t)
	scripts, err := live.UserScripts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) != 1 {
		t.Fatalf("expected one script, got %d", len(scripts))
	}
	script := scripts[0]
	if script.Name != "Nightly backup" || script.Frequency != "custom" || script.Cron != "0 3 * * *" {
		t.Fatalf("unexpected script: %+v", script)
	}
	if script.Content == "" {
		t.Fatal("expected script content")
	}
}

func TestUpdateUserScriptWritesScheduleAndCron(t *testing.T) {
	live := userScriptTestProvider(t)
	if err := live.UpdateUserScript(context.Background(), "backup", "#!/bin/bash\necho updated\n", "daily", ""); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(live.bootDir, "config", "plugins", "user.scripts", "schedule.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"frequency": "daily"`) {
		t.Fatalf("schedule.json was not updated: %s", raw)
	}
	cronPath := filepath.Join(live.bootDir, "config", "plugins", "user.scripts", "customSchedule.cron")
	if _, err := os.Stat(cronPath); !os.IsNotExist(err) {
		t.Fatal("custom cron file should be removed for non-custom schedules")
	}
	if err := live.UpdateUserScript(context.Background(), "backup", "#!/bin/bash\n", "custom", "*/15 * * * *"); err != nil {
		t.Fatal(err)
	}
	cron, err := os.ReadFile(cronPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cron), "*/15 * * * *") || !strings.Contains(string(cron), "startCustom.php") {
		t.Fatalf("unexpected cron fragment: %s", cron)
	}
	if err := live.UpdateUserScript(context.Background(), "backup", "x", "custom", "bad cron"); err == nil {
		t.Fatal("expected an invalid cron expression to be rejected")
	}
}

func TestValidCronExpression(t *testing.T) {
	valid := []string{"0 3 * * *", "*/15 * * * *", "0 0 1 * *"}
	for _, value := range valid {
		if !validCronExpression(value) {
			t.Fatalf("expected %q to be valid", value)
		}
	}
	invalid := []string{"", "0 3 * *", "0 3 * * * *", "bad cron", "0 3 * * $(reboot)"}
	for _, value := range invalid {
		if validCronExpression(value) {
			t.Fatalf("expected %q to be invalid", value)
		}
	}
}
