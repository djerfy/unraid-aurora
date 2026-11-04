package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurora/internal/model"
)

func TestParseCAFeedBuildsCatalog(t *testing.T) {
	raw := []byte(`{
	  "last_updated_timestamp": 1790000000,
	  "applist": [
	    {"Name":"Example","Repository":"example/app:latest","TemplateURL":"https://example.test/app.xml","CategoryList":["Tools","Media"],"downloads":42,"Privileged":"false"},
	    {"Name":"Plugin","Plugin":true,"PluginURL":"https://example.test/plugin.plg","CategoryList":["Plugins"],"PluginAuthor":"Author"}
	  ]
	}`)
	entries, categories, _, err := parseCAFeed(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Kind != "docker" || entries[1].Kind != "plugin" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	if entries[0].Downloads != 42 || entries[1].Author != "Author" {
		t.Fatalf("metadata was not mapped: %+v", entries)
	}
	if len(categories) != 3 {
		t.Fatalf("unexpected categories: %+v", categories)
	}
}

func TestValidateHTTPSURL(t *testing.T) {
	for _, target := range []string{"http://example.test/a", "file:///etc/passwd", "https://user:pass@example.test/a", "not a url"} {
		if err := validateHTTPSURL(target); err == nil {
			t.Fatalf("%q should be rejected", target)
		}
	}
	if err := validateHTTPSURL("https://example.test/a"); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeSchedulerDefaultsMissingFields(t *testing.T) {
	settings := model.SchedulerSettings{
		Mover: model.MoverSchedule{SchedulerJob: model.SchedulerJob{Mode: "daily"}},
		Trim:  model.TrimSchedule{SchedulerJob: model.SchedulerJob{Mode: "hourly"}},
	}
	normalizeSchedulerSettings(&settings)
	if settings.Mover.Month != 1 || settings.Mover.DayOfMonth != 1 || settings.Mover.IntervalHours != 1 {
		t.Fatalf("mover defaults were not normalized: %+v", settings.Mover)
	}
	if settings.Trim.Month != 1 || settings.Trim.DayOfMonth != 1 || settings.Trim.IntervalHours != 1 {
		t.Fatalf("trim defaults were not normalized: %+v", settings.Trim)
	}
}

func TestPreviewSchedulerAcceptsMissingMonthAndInterval(t *testing.T) {
	live := &Live{bootDir: t.TempDir()}
	_, err := live.PreviewScheduler(model.SchedulerSettings{
		Parity: model.ParitySchedule{SchedulerJob: model.SchedulerJob{Mode: "daily", Hour: 3, Minute: 0}},
		Mover:  model.MoverSchedule{SchedulerJob: model.SchedulerJob{Mode: "daily", Hour: 4, Minute: 0}},
		Trim:   model.TrimSchedule{SchedulerJob: model.SchedulerJob{Mode: "hourly", IntervalHours: 0}},
	})
	if err != nil {
		t.Fatalf("preview should normalize omitted month/interval fields: %v", err)
	}
}

func TestUpsertINIPreservesCRLF(t *testing.T) {
	input := "shareMoverSchedule=\"30 3 * * *\"\r\nshareMoverLogging=\"yes\"\r\n"
	output := upsertINIValues(input, "", map[string]string{
		"shareMoverSchedule": "30 3 * * *",
		"shareMoverLogging":  "yes",
	})
	if output != input {
		t.Fatalf("CRLF content changed:\n%q\n%q", input, output)
	}
	if diff := renderConfigDiff("share.cfg", []byte(input), []byte(output), false); diff != "" {
		t.Fatalf("unchanged CRLF content produced a diff:\n%s", diff)
	}
}

func TestPreviewSchedulerDisabledTrimDoesNotMaterialiseSection(t *testing.T) {
	boot := t.TempDir()
	dynamixDir := filepath.Join(boot, "config/plugins/dynamix")
	if err := os.MkdirAll(dynamixDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dynamixDir, "dynamix.cfg")
	original := "[parity]\nmode=\"0\"\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	live := &Live{bootDir: boot}
	settings := model.SchedulerSettings{
		Parity: model.ParitySchedule{SchedulerJob: model.SchedulerJob{Mode: "disabled", Hour: 3, DayOfMonth: 1, Month: 1, IntervalHours: 1}},
		Mover:  model.MoverSchedule{SchedulerJob: model.SchedulerJob{Mode: "disabled", Hour: 3, DayOfMonth: 1, Month: 1, IntervalHours: 1}},
		Trim:   model.TrimSchedule{SchedulerJob: model.SchedulerJob{Mode: "disabled", Hour: 3, DayOfMonth: 1, Month: 1, IntervalHours: 1}},
	}
	preview, err := live.PreviewScheduler(settings)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range preview.Files {
		if file.Path == path {
			if strings.Contains(file.Content, "[ssd]") {
				t.Fatalf("disabled TRIM created an ssd section:\n%s", file.Content)
			}
		}
	}
}

func TestRunCronHelperHandlesShebanglessScript(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update_cron")
	// Unraid ships update_cron as "#/bin/bash" (no "!"), which cannot be
	// execve'd directly.
	if err := os.WriteFile(path, []byte("#/bin/bash\necho cron-ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	output, err := runCronHelper(context.Background(), path)
	if err != nil {
		t.Fatalf("run update_cron: %v", err)
	}
	if !strings.Contains(string(output), "cron-ok") {
		t.Fatalf("unexpected output: %q", output)
	}
}

func TestMoverCronMatchesOfficialFragment(t *testing.T) {
	got := moverCron(model.MoverSchedule{SchedulerJob: model.SchedulerJob{Mode: "daily", Hour: 3, Minute: 30}})
	want := "# Generated mover schedule:\n30 3 * * * /usr/local/sbin/mover start |& logger -t move\n\n"
	if got != want {
		t.Fatalf("unexpected mover cron:\n%q\n%q", want, got)
	}
	if diff := renderConfigDiff("mover.cron", []byte(want), []byte(got), false); diff != "" {
		t.Fatalf("identical mover cron produced a diff:\n%s", diff)
	}
}

func TestPreviewDisabledParityKeepsInactiveFields(t *testing.T) {
	boot := t.TempDir()
	dynamixDir := filepath.Join(boot, "config/plugins/dynamix")
	if err := os.MkdirAll(dynamixDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dynamixDir, "dynamix.cfg")
	original := "[parity]\nmode=\"0\"\nhour=\"0 3\"\nday=\"0\"\ndotm=\"1\"\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	live := &Live{bootDir: boot}
	settings := model.SchedulerSettings{
		Parity: model.ParitySchedule{SchedulerJob: model.SchedulerJob{Mode: "disabled", Hour: 4, Minute: 30, Day: 2, DayOfMonth: 5, Month: 7, IntervalHours: 1}},
		Mover:  model.MoverSchedule{SchedulerJob: model.SchedulerJob{Mode: "disabled", Hour: 3, DayOfMonth: 1, Month: 1, IntervalHours: 1}},
		Trim:   model.TrimSchedule{SchedulerJob: model.SchedulerJob{Mode: "disabled", Hour: 3, DayOfMonth: 1, Month: 1, IntervalHours: 1}},
	}
	preview, err := live.PreviewScheduler(settings)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range preview.Files {
		if file.Path != path {
			continue
		}
		if strings.Contains(file.Content, "month=") || strings.Contains(file.Content, "write=") {
			t.Fatalf("disabled parity materialised inactive fields:\n%s", file.Content)
		}
		if file.Diff != "" {
			t.Fatalf("unchanged disabled parity produced a diff:\n%s", file.Diff)
		}
	}
}

func TestSchedulerReadPreviewAndUpsert(t *testing.T) {
	boot := t.TempDir()
	dynamixDir := filepath.Join(boot, "config/plugins/dynamix")
	if err := os.MkdirAll(dynamixDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dynamix := `# keep this comment
[parity]
mode="2"
hour="30 3"
day="1"
dotm="1"
month="1"
write=""
[display]
date="%A, %Y-%m-%d"
time="%R"
[unknown]
value="keep"
`
	if err := os.WriteFile(filepath.Join(dynamixDir, "dynamix.cfg"), []byte(dynamix), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(boot, "config/share.cfg"), []byte("shareMoverSchedule=\"0 4 * * 1\"\nshareMoverLogging=\"yes\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	live := &Live{bootDir: boot}
	settings, err := live.readSchedulerSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.Parity.Mode != "weekly" || settings.Parity.Day != 1 || settings.Parity.Hour != 3 || !settings.Parity.WriteCorrections {
		t.Fatalf("unexpected parity settings: %+v", settings.Parity)
	}
	if settings.Mover.Mode != "weekly" || settings.Mover.Day != 1 || !settings.Mover.Logging {
		t.Fatalf("unexpected mover settings: %+v", settings.Mover)
	}

	preview, err := live.PreviewScheduler(model.SchedulerSettings{
		Parity: model.ParitySchedule{SchedulerJob: model.SchedulerJob{Mode: "daily", Hour: 4, Minute: 15, Day: 0, DayOfMonth: 1, Month: 1, IntervalHours: 1}, WriteCorrections: false},
		Mover:  model.MoverSchedule{SchedulerJob: model.SchedulerJob{Mode: "monthly", Hour: 5, Minute: 0, Day: 0, DayOfMonth: 2, Month: 1, IntervalHours: 1}, Logging: true},
		Trim:   model.TrimSchedule{SchedulerJob: model.SchedulerJob{Mode: "hourly", Hour: 0, Minute: 0, Day: 0, DayOfMonth: 1, Month: 1, IntervalHours: 6}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Files) != 5 {
		t.Fatalf("unexpected preview files: %+v", preview.Files)
	}
	joined := ""
	for _, file := range preview.Files {
		joined += file.Path + "\n" + file.Content + "\n" + file.Diff
	}
	for _, expected := range []string{"15 4 * * *", "0 5 2 * *", "0 */6 * * *", "[unknown]", "keep", "# Generated mover schedule:\n0 5 2 * * /usr/local/sbin/mover start |& logger -t move\n"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("preview is missing %q:\n%s", expected, joined)
		}
	}
	if !strings.Contains(joined, "@@") || !strings.Contains(joined, "+") {
		t.Fatalf("preview diff is missing:\n%s", joined)
	}
	for _, file := range preview.Files {
		if strings.HasSuffix(file.Path, "mover.cron") && strings.Contains(file.Content, "logger &> /dev/null") {
			t.Fatalf("mover cron lost the logger tag:\n%s", file.Content)
		}
	}

	upserted := upsertINIValues(dynamix, "parity", map[string]string{"mode": "1", "hour": "0 6"})
	if !strings.Contains(upserted, "# keep this comment") || !strings.Contains(upserted, "[unknown]") {
		t.Fatalf("upsert dropped unrelated content:\n%s", upserted)
	}
	if !strings.Contains(upserted, `mode="1"`) || !strings.Contains(upserted, `hour="0 6"`) {
		t.Fatalf("upsert did not replace values:\n%s", upserted)
	}
}
