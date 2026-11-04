package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"aurora/internal/model"
)

const (
	updateCronPath = "/usr/local/sbin/update_cron"
	mdcmdPath      = "/usr/local/sbin/mdcmd"
	moverPath      = "/usr/local/sbin/mover"
	trimPath       = "/usr/local/emhttp/plugins/dynamix/scripts/ssd_trim"
)

var displayFormatPattern = regexp.MustCompile(`^[%A-Za-z0-9 ,./:_+-]{1,40}$`)

func defaultSchedulerSettings() model.SchedulerSettings {
	base := model.SchedulerJob{Mode: "disabled", Hour: 3, Minute: 0, Day: 0, DayOfMonth: 1, Month: 1, IntervalHours: 1}
	return model.SchedulerSettings{
		Parity: model.ParitySchedule{SchedulerJob: base, WriteCorrections: false},
		Mover:  model.MoverSchedule{SchedulerJob: base, Logging: false, Progress: false},
		Trim:   model.TrimSchedule{SchedulerJob: base},
	}
}

func (l *Live) dynamixConfigPath() string {
	if l.bootDir == "" {
		return ""
	}
	return filepath.Join(l.bootDir, "config/plugins/dynamix/dynamix.cfg")
}

func (l *Live) schedulerShareConfigPath() string {
	if l.bootDir == "" {
		return ""
	}
	return filepath.Join(l.bootDir, "config/share.cfg")
}

func (l *Live) schedulerCronPath(name string) string {
	if l.bootDir == "" {
		return ""
	}
	return filepath.Join(l.bootDir, "config/plugins/dynamix", name)
}

func (l *Live) SchedulerState(ctx context.Context) (model.SchedulerState, error) {
	settings, err := l.readSchedulerSettings()
	if err != nil {
		return model.SchedulerState{}, err
	}
	system := model.SystemSettings{CurrentTime: time.Now(), NTPServers: []string{}}
	if l.stateDir != "" {
		if ini, err := parseINI(filepath.Join(l.stateDir, "var.ini")); err == nil {
			system.TimeZone = ini.get("", "timeZone")
			system.NTPEnabled = strings.EqualFold(ini.get("", "USE_NTP"), "yes")
			for _, key := range []string{"NTP_SERVER1", "NTP_SERVER2", "NTP_SERVER3", "NTP_SERVER4"} {
				if value := strings.TrimSpace(ini.get("", key)); value != "" {
					system.NTPServers = append(system.NTPServers, value)
				}
			}
		}
	}
	system.Display = l.readDisplaySettings()
	return model.SchedulerState{Settings: settings, System: system}, nil
}

func (l *Live) readDisplaySettings() model.SystemDisplaySettings {
	display := model.SystemDisplaySettings{DateFormat: "%c", TimeFormat: "%R"}
	path := l.dynamixConfigPath()
	if path == "" {
		return display
	}
	if ini, err := parseINI(path); err == nil {
		if value := ini.get("display", "date"); value != "" {
			display.DateFormat = value
		}
		if value := ini.get("display", "time"); value != "" {
			display.TimeFormat = value
		}
	}
	return display
}

func (l *Live) readSchedulerSettings() (model.SchedulerSettings, error) {
	settings := defaultSchedulerSettings()
	dynamixPath := l.dynamixConfigPath()
	if dynamixPath == "" {
		return settings, errors.New("the flash directory is not configured")
	}
	if ini, err := parseINI(dynamixPath); err == nil {
		settings.Parity = parseParitySchedule(ini.section("parity"))
		settings.Trim = model.TrimSchedule{SchedulerJob: parseTrimSchedule(ini.section("ssd"))}
	}
	if ini, err := parseINI(l.schedulerShareConfigPath()); err == nil {
		settings.Mover = parseMoverSchedule(ini.section(""))
	}
	return settings, nil
}

func parseParitySchedule(section map[string]string) model.ParitySchedule {
	job := model.SchedulerJob{Mode: "disabled", Hour: 3, DayOfMonth: 1, Month: 1, IntervalHours: 1}
	if section == nil {
		return model.ParitySchedule{SchedulerJob: job}
	}
	job.Mode = parityModeName(atoi(section["mode"]))
	job.Minute, job.Hour = parseClock(section["hour"])
	job.Day = atoi(section["day"])
	job.DayOfMonth = atoi(section["dotm"])
	if job.DayOfMonth < 1 || job.DayOfMonth > 31 {
		job.DayOfMonth = 1
	}
	job.Month = atoi(section["month"])
	if job.Month < 1 || job.Month > 12 {
		job.Month = 1
	}
	return model.ParitySchedule{SchedulerJob: job, WriteCorrections: strings.TrimSpace(section["write"]) == ""}
}

func parseTrimSchedule(section map[string]string) model.SchedulerJob {
	job := model.SchedulerJob{Mode: "disabled", Hour: 3, DayOfMonth: 1, Month: 1, IntervalHours: 1}
	if section == nil {
		return job
	}
	job.Mode = trimModeName(atoi(section["mode"]))
	job.Day = atoi(section["day"])
	job.DayOfMonth = atoi(section["dotm"])
	if job.DayOfMonth < 1 || job.DayOfMonth > 31 {
		job.DayOfMonth = 1
	}
	if job.Mode == "hourly" {
		job.IntervalHours = parseCronInterval(section["hour"])
		job.Hour, job.Minute = 0, 0
	} else {
		job.Hour = atoi(section["hour"])
		job.Minute = atoi(section["min"])
	}
	return job
}

func parseMoverSchedule(section map[string]string) model.MoverSchedule {
	job := model.SchedulerJob{Mode: "disabled", Hour: 3, DayOfMonth: 1, Month: 1, IntervalHours: 1}
	mover := model.MoverSchedule{SchedulerJob: job}
	if section == nil {
		return mover
	}
	mover.Logging = strings.EqualFold(section["shareMoverLogging"], "yes")
	mover.Progress = strings.EqualFold(section["shareMoverProgress"], "yes")
	cron := strings.Fields(section["shareMoverSchedule"])
	if len(cron) < 5 {
		return mover
	}
	job.Minute = atoi(cron[0])
	job.IntervalHours = parseCronInterval(cron[1])
	if job.IntervalHours > 0 {
		job.Mode = "hourly"
		job.Hour, job.Minute = 0, 0
	} else if cron[4] != "*" {
		job.Mode = "weekly"
		job.Day = atoi(cron[4])
		job.Hour = atoi(cron[1])
	} else if cron[2] != "*" {
		job.Mode = "monthly"
		job.DayOfMonth = atoi(cron[2])
		job.Hour = atoi(cron[1])
	} else {
		job.Mode = "daily"
		job.Hour = atoi(cron[1])
	}
	mover.SchedulerJob = job
	return mover
}

func parityModeName(value int) string {
	switch value {
	case 1:
		return "daily"
	case 2:
		return "weekly"
	case 3:
		return "monthly"
	case 4:
		return "yearly"
	default:
		return "disabled"
	}
}

func trimModeName(value int) string {
	switch value {
	case 1:
		return "hourly"
	case 2:
		return "daily"
	case 3:
		return "weekly"
	case 4:
		return "monthly"
	default:
		return "disabled"
	}
}

func parityModeNumber(mode string) int {
	switch mode {
	case "daily":
		return 1
	case "weekly":
		return 2
	case "monthly":
		return 3
	case "yearly":
		return 4
	default:
		return 0
	}
}

func trimModeNumber(mode string) int {
	switch mode {
	case "hourly":
		return 1
	case "daily":
		return 2
	case "weekly":
		return 3
	case "monthly":
		return 4
	default:
		return 0
	}
}

func parseClock(value string) (int, int) {
	fields := strings.Fields(value)
	if len(fields) != 2 {
		return 0, 0
	}
	return atoi(fields[0]), atoi(fields[1])
}

func parseCronInterval(value string) int {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "*/") {
		interval := atoi(strings.TrimPrefix(value, "*/"))
		if interval > 0 && interval < 24 {
			return interval
		}
	}
	return 0
}

func (l *Live) PreviewScheduler(settings model.SchedulerSettings) (model.SchedulerPreview, error) {
	if l.bootDir == "" {
		return model.SchedulerPreview{}, errors.New("the flash directory is not configured")
	}
	normalizeSchedulerSettings(&settings)
	if err := validateSchedulerSettings(settings); err != nil {
		return model.SchedulerPreview{}, err
	}
	files := []model.SchedulerPreviewFile{}
	dynamix, _ := os.ReadFile(l.dynamixConfigPath())
	share, _ := os.ReadFile(l.schedulerShareConfigPath())

	dynamixText := string(dynamix)
	if settings.Parity.Mode == "disabled" {
		// The official parity page disables hour/day/dotm/month/write, so those
		// fields are not submitted. Mirror that: only flip the mode and keep the
		// inactive values untouched to avoid a noisy preview.
		if iniSectionExists(dynamixText, "parity") {
			dynamixText = upsertINIValues(dynamixText, "parity", map[string]string{"mode": "0"})
		}
	} else {
		dynamixText = upsertINIValues(dynamixText, "parity", map[string]string{
			"mode":  strconv.Itoa(parityModeNumber(settings.Parity.Mode)),
			"hour":  fmt.Sprintf("%d %d", settings.Parity.Minute, settings.Parity.Hour),
			"day":   strconv.Itoa(settings.Parity.Day),
			"dotm":  strconv.Itoa(settings.Parity.DayOfMonth),
			"month": strconv.Itoa(settings.Parity.Month),
			"write": boolParityWrite(settings.Parity.WriteCorrections),
		})
	}
	if settings.Trim.Mode == "disabled" {
		// Do not materialise a disabled TRIM section. If the user already has
		// one, only switch its mode and keep the inactive values untouched.
		if iniSectionExists(dynamixText, "ssd") {
			dynamixText = upsertINIValues(dynamixText, "ssd", map[string]string{"mode": "0"})
		}
	} else {
		dynamixText = upsertINIValues(dynamixText, "ssd", map[string]string{
			"mode": strconv.Itoa(trimModeNumber(settings.Trim.Mode)),
			"hour": trimHour(settings.Trim.SchedulerJob),
			"min":  strconv.Itoa(settings.Trim.Minute),
			"day":  strconv.Itoa(settings.Trim.Day),
			"dotm": strconv.Itoa(settings.Trim.DayOfMonth),
		})
	}
	files = append(files, previewFile(l.dynamixConfigPath(), dynamixText))

	shareText := upsertINIValues(string(share), "", map[string]string{
		"shareMoverSchedule": moverCronSchedule(settings.Mover),
		"shareMoverLogging":  yesNo(settings.Mover.Logging),
		"shareMoverProgress": yesNo(settings.Mover.Progress),
	})
	files = append(files, previewFile(l.schedulerShareConfigPath(), shareText))

	cronFiles := []struct {
		name    string
		content string
	}{
		{"parity-check.cron", parityCron(settings.Parity)},
		{"mover.cron", moverCron(settings.Mover)},
		{"ssd-trim.cron", trimCron(settings.Trim)},
	}
	warnings := []string{}
	if _, err := os.Stat(updateCronPath); err != nil {
		warnings = append(warnings, "update_cron is not available; the schedule will be written but not activated")
	}
	for _, cron := range cronFiles {
		path := l.schedulerCronPath(cron.name)
		exists := false
		if _, err := os.Stat(path); err == nil {
			exists = true
		}
		current, _ := os.ReadFile(path)
		diff := renderConfigDiff(path, current, []byte(cron.content), cron.content == "")
		files = append(files, model.SchedulerPreviewFile{
			Path: path, Content: cron.content, Diff: diff, Existed: exists, Delete: cron.content == "",
		})
	}
	return model.SchedulerPreview{Files: files, Warnings: warnings}, nil
}

func (l *Live) ApplyScheduler(ctx context.Context, settings model.SchedulerSettings) error {
	preview, err := l.PreviewScheduler(settings)
	if err != nil {
		return err
	}
	changes := make([]configChange, 0, len(preview.Files))
	for _, file := range preview.Files {
		current, readErr := os.ReadFile(file.Path)
		exists := readErr == nil
		if readErr != nil && !os.IsNotExist(readErr) {
			return fmt.Errorf("read %s: %w", file.Path, readErr)
		}
		if file.Delete {
			if !exists {
				continue
			}
			changes = append(changes, configChange{Path: file.Path, Delete: true})
			continue
		}
		// Avoid needless flash writes when the generated file is already current.
		if exists && string(current) == file.Content {
			continue
		}
		changes = append(changes, configChange{Path: file.Path, Data: []byte(file.Content)})
	}
	if err := l.writeConfigChanges("scheduler.apply", "Update parity, mover and TRIM schedules", changes); err != nil {
		return err
	}
	output, err := runCronHelper(ctx, updateCronPath)
	if err != nil {
		return fmt.Errorf("activate cron schedule: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// runCronHelper executes Unraid's update_cron helper. The shipped script starts
// with "#/bin/bash" (the leading "!" is missing), so a direct execve fails with
// ENOEXEC. Running it through bash matches what the WebGUI does via exec().
func runCronHelper(ctx context.Context, path string) ([]byte, error) {
	if _, err := os.Stat("/bin/bash"); err == nil {
		return exec.CommandContext(ctx, "/bin/bash", path).CombinedOutput()
	}
	return exec.CommandContext(ctx, path).CombinedOutput()
}

func (l *Live) UpdateDisplaySettings(ctx context.Context, settings model.SystemDisplaySettings) error {
	if l.bootDir == "" {
		return errors.New("the flash directory is not configured")
	}
	if !displayFormatPattern.MatchString(settings.DateFormat) || !displayFormatPattern.MatchString(settings.TimeFormat) {
		return errors.New("invalid date or time format")
	}
	path := l.dynamixConfigPath()
	raw, _ := os.ReadFile(path)
	text := upsertINIValues(string(raw), "display", map[string]string{
		"date": settings.DateFormat,
		"time": settings.TimeFormat,
	})
	return l.writeConfigFile("display.update", "Update date and time display formats", path, []byte(text))
}

func normalizeSchedulerSettings(settings *model.SchedulerSettings) {
	normalizeSchedulerJob(&settings.Parity.SchedulerJob)
	normalizeSchedulerJob(&settings.Mover.SchedulerJob)
	normalizeSchedulerJob(&settings.Trim.SchedulerJob)
}

func normalizeSchedulerJob(job *model.SchedulerJob) {
	if job.DayOfMonth < 1 {
		job.DayOfMonth = 1
	}
	if job.Month < 1 {
		job.Month = 1
	}
	if job.IntervalHours < 1 {
		job.IntervalHours = 1
	}
}

func validateSchedulerSettings(settings model.SchedulerSettings) error {
	if err := validateJob(settings.Parity.SchedulerJob, false); err != nil {
		return fmt.Errorf("parity: %w", err)
	}
	if err := validateJob(settings.Mover.SchedulerJob, true); err != nil {
		return fmt.Errorf("mover: %w", err)
	}
	if err := validateJob(settings.Trim.SchedulerJob, true); err != nil {
		return fmt.Errorf("trim: %w", err)
	}
	return nil
}

func validateJob(job model.SchedulerJob, allowHourly bool) error {
	valid := map[string]bool{"disabled": true, "daily": true, "weekly": true, "monthly": true}
	if allowHourly {
		valid["hourly"] = true
	} else {
		valid["yearly"] = true
	}
	if !valid[job.Mode] {
		return fmt.Errorf("unsupported schedule mode %q", job.Mode)
	}
	if job.Hour < 0 || job.Hour > 23 || job.Minute < 0 || job.Minute > 59 {
		return errors.New("hour must be 0-23 and minute 0-59")
	}
	if job.Day < 0 || job.Day > 6 {
		return errors.New("day must be 0-6")
	}
	if job.DayOfMonth < 1 || job.DayOfMonth > 31 {
		return errors.New("day of month must be 1-31")
	}
	if job.Month < 1 || job.Month > 12 {
		return errors.New("month must be 1-12")
	}
	if job.IntervalHours < 1 || job.IntervalHours > 23 {
		return errors.New("hour interval must be 1-23")
	}
	return nil
}

func previewFile(path, content string) model.SchedulerPreviewFile {
	current, err := os.ReadFile(path)
	exists := err == nil
	return model.SchedulerPreviewFile{
		Path: path, Content: content, Existed: exists,
		Diff: renderConfigDiff(path, current, []byte(content), false),
	}
}

func boolParityWrite(correct bool) string {
	if correct {
		return ""
	}
	return "NOCORRECT"
}

func trimHour(job model.SchedulerJob) string {
	if job.Mode == "hourly" {
		return fmt.Sprintf("*/%d", max(1, job.IntervalHours))
	}
	return strconv.Itoa(job.Hour)
}

func moverCronSchedule(job model.MoverSchedule) string {
	switch job.Mode {
	case "hourly":
		return fmt.Sprintf("0 */%d * * *", max(1, job.IntervalHours))
	case "daily":
		return fmt.Sprintf("%d %d * * *", job.Minute, job.Hour)
	case "weekly":
		return fmt.Sprintf("%d %d * * %d", job.Minute, job.Hour, job.Day)
	case "monthly":
		return fmt.Sprintf("%d %d %d * *", job.Minute, job.Hour, job.DayOfMonth)
	default:
		return ""
	}
}

func parityCron(job model.ParitySchedule) string {
	write := "NOCORRECT"
	if job.WriteCorrections {
		write = ""
	}
	switch job.Mode {
	case "daily":
		return fmt.Sprintf("# Generated parity check schedule:\n%d %d * * * %s check %s &> /dev/null\n", job.Minute, job.Hour, mdcmdPath, write)
	case "weekly":
		return fmt.Sprintf("# Generated parity check schedule:\n%d %d * * %d %s check %s &> /dev/null\n", job.Minute, job.Hour, job.Day, mdcmdPath, write)
	case "monthly":
		return fmt.Sprintf("# Generated parity check schedule:\n%d %d %d * * %s check %s &> /dev/null\n", job.Minute, job.Hour, job.DayOfMonth, mdcmdPath, write)
	case "yearly":
		return fmt.Sprintf("# Generated parity check schedule:\n%d %d %d %d * %s check %s &> /dev/null\n", job.Minute, job.Hour, job.DayOfMonth, job.Month, mdcmdPath, write)
	default:
		return ""
	}
}

func moverCron(job model.MoverSchedule) string {
	schedule := moverCronSchedule(job)
	if schedule == "" {
		return ""
	}
	// Emhttp writes the fragment with a trailing blank line; keeping it avoids a
	// spurious diff on every preview.
	return fmt.Sprintf("# Generated mover schedule:\n%s %s start |& logger -t move\n\n", schedule, moverPath)
}

func trimCron(job model.TrimSchedule) string {
	switch job.Mode {
	case "hourly":
		return fmt.Sprintf("# Generated TRIM schedule:\n0 */%d * * * %s cron|logger &> /dev/null\n", max(1, job.IntervalHours), trimPath)
	case "daily":
		return fmt.Sprintf("# Generated TRIM schedule:\n%d %d * * * %s cron|logger &> /dev/null\n", job.Minute, job.Hour, trimPath)
	case "weekly":
		return fmt.Sprintf("# Generated TRIM schedule:\n%d %d * * %d %s cron|logger &> /dev/null\n", job.Minute, job.Hour, job.Day, trimPath)
	case "monthly":
		return fmt.Sprintf("# Generated TRIM schedule:\n%d %d %d * * %s cron|logger &> /dev/null\n", job.Minute, job.Hour, job.DayOfMonth, trimPath)
	default:
		return ""
	}
}

// iniSectionExists reports whether a named section is already present.
func iniSectionExists(raw, sectionName string) bool {
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if strings.Trim(strings.TrimSpace(trimmed[1:len(trimmed)-1]), `"'`) == sectionName {
				return true
			}
		}
	}
	return false
}

// upsertINIValues updates keys inside a section while preserving comments and
// unknown keys. An empty section name targets the top-level block.
func upsertINIValues(raw, sectionName string, values map[string]string) string {
	crlf := strings.Contains(raw, "\r\n")
	if crlf {
		raw = strings.ReplaceAll(raw, "\r\n", "\n")
	}
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = []string{}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	sectionStart := -1
	sectionEnd := len(lines)
	current := ""
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if current == sectionName && sectionStart >= 0 {
				sectionEnd = index
				break
			}
			current = strings.Trim(strings.TrimSpace(trimmed[1:len(trimmed)-1]), `"'`)
			if current == sectionName {
				sectionStart = index
			}
			continue
		}
		if sectionName == "" && sectionStart < 0 && !strings.HasPrefix(trimmed, "[") {
			sectionStart = 0
		}
	}
	if sectionName == "" {
		if sectionStart < 0 {
			sectionStart = 0
		}
		if sectionStart == 0 && len(lines) > 0 {
			for index, line := range lines {
				if strings.HasPrefix(strings.TrimSpace(line), "[") {
					sectionEnd = index
					break
				}
			}
		}
	}
	if sectionStart < 0 {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		if sectionName != "" {
			lines = append(lines, "["+sectionName+"]")
		}
		for _, key := range keys {
			lines = append(lines, key+"="+iniQuote(values[key]))
		}
		result := strings.Join(lines, "\n") + "\n"
		if crlf {
			result = strings.ReplaceAll(result, "\n", "\r\n")
		}
		return result
	}

	seen := map[string]bool{}
	scanStart := sectionStart + 1
	if sectionName == "" {
		scanStart = sectionStart
	}
	for index := scanStart; index < sectionEnd && index < len(lines); index++ {
		line := lines[index]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, _, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if value, exists := values[key]; exists {
			lines[index] = key + "=" + iniQuote(value)
			seen[key] = true
		}
	}
	insertAt := sectionEnd
	missing := []string{}
	for _, key := range keys {
		if !seen[key] {
			missing = append(missing, key+"="+iniQuote(values[key]))
		}
	}
	if len(missing) > 0 {
		lines = append(lines[:insertAt], append(missing, lines[insertAt:]...)...)
	}
	result := strings.Join(lines, "\n") + "\n"
	if crlf {
		result = strings.ReplaceAll(result, "\n", "\r\n")
	}
	return result
}

func iniQuote(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
