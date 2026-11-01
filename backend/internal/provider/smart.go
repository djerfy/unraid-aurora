package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"aurora/internal/model"
)

var deviceNamePattern = regexp.MustCompile(`^[A-Za-z0-9]+$`)

type smartctlPayload struct {
	ModelName       string `json:"model_name"`
	ModelFamily     string `json:"model_family"`
	SerialNumber    string `json:"serial_number"`
	FirmwareVersion string `json:"firmware_version"`
	SmartStatus     struct {
		Passed bool `json:"passed"`
	} `json:"smart_status"`
	Temperature struct {
		Current int `json:"current"`
	} `json:"temperature"`
	PowerOnTime struct {
		Hours int `json:"hours"`
	} `json:"power_on_time"`
	AtaSmartAttributes struct {
		Table []struct {
			ID         int    `json:"id"`
			Name       string `json:"name"`
			Value      int    `json:"value"`
			Worst      int    `json:"worst"`
			Threshold  int    `json:"thresh"`
			WhenFailed string `json:"when_failed"`
			Raw        struct {
				Value  int    `json:"value"`
				String string `json:"string"`
			} `json:"raw"`
		} `json:"table"`
	} `json:"ata_smart_attributes"`
	AtaSmartSelfTestLog struct {
		Standard []struct {
			Status struct {
				String string `json:"string"`
				Passed bool   `json:"passed"`
			} `json:"status"`
		} `json:"standard"`
	} `json:"ata_smart_self_test_log"`
	NVMeSmartHealth struct {
		Temperature    int `json:"temperature"`
		PowerOnHours   int `json:"power_on_hours"`
		PercentageUsed int `json:"percentage_used"`
		MediaErrors    int `json:"media_errors"`
		CriticalWarn   int `json:"critical_warning"`
	} `json:"nvme_smart_health_information_log"`
	Messages []struct {
		String string `json:"string"`
	} `json:"messages"`
}

func smartctlBinary() string {
	for _, candidate := range []string{"/usr/sbin/smartctl", "/usr/bin/smartctl", "/sbin/smartctl"} {
		if isExecutable(candidate) {
			return candidate
		}
	}
	if path, err := exec.LookPath("smartctl"); err == nil {
		return path
	}
	return ""
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

// SmartReport reads health, temperature and SMART attributes through smartctl.
// A missing smartctl or an unsupported device is reported in the payload rather
// than treated as a hard error, so the disk drawer can explain the situation.
func (l *Live) SmartReport(ctx context.Context, device string) (model.SmartReport, error) {
	report := model.SmartReport{Device: device, Attributes: []model.SmartAttribute{}}
	device = strings.TrimSpace(device)
	if !deviceNamePattern.MatchString(device) {
		return report, errors.New("invalid disk device")
	}
	binary := smartctlBinary()
	if binary == "" {
		report.Message = "smartctl is not installed on this host"
		return report, nil
	}
	commandCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	raw, err := exec.CommandContext(commandCtx, binary, "-H", "-A", "-i", "-j", "/dev/"+device).Output()
	if len(raw) == 0 {
		if err != nil {
			return report, fmt.Errorf("smartctl: %w", err)
		}
		return report, errors.New("smartctl returned no data")
	}
	parsed, err := parseSmartctlPayload(raw)
	if err != nil {
		report.Message = "smartctl output could not be parsed"
		return report, nil
	}
	parsed.Device = device
	return parsed, nil
}

func parseSmartctlPayload(raw []byte) (model.SmartReport, error) {
	report := model.SmartReport{Attributes: []model.SmartAttribute{}}
	var payload smartctlPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return report, err
	}
	report.Available = true
	report.Model = payload.ModelName
	if report.Model == "" {
		report.Model = payload.ModelFamily
	}
	report.Serial = payload.SerialNumber
	report.Firmware = payload.FirmwareVersion
	report.Passed = payload.SmartStatus.Passed
	report.Temperature = payload.Temperature.Current
	if report.Temperature == 0 && payload.NVMeSmartHealth.Temperature > 0 {
		value := payload.NVMeSmartHealth.Temperature
		if value > 200 {
			value -= 273
		}
		report.Temperature = value
	}
	report.PowerOnHours = payload.PowerOnTime.Hours
	if report.PowerOnHours == 0 {
		report.PowerOnHours = payload.NVMeSmartHealth.PowerOnHours
	}
	if len(payload.AtaSmartSelfTestLog.Standard) > 0 {
		report.SelfTest = payload.AtaSmartSelfTestLog.Standard[0].Status.String
	}
	for _, attribute := range payload.AtaSmartAttributes.Table {
		rawValue := attribute.Raw.String
		if rawValue == "" {
			rawValue = strconv.Itoa(attribute.Raw.Value)
		}
		report.Attributes = append(report.Attributes, model.SmartAttribute{
			ID:        attribute.ID,
			Name:      attribute.Name,
			Value:     attribute.Value,
			Worst:     attribute.Worst,
			Threshold: attribute.Threshold,
			Raw:       rawValue,
			Failing:   attribute.WhenFailed != "",
		})
	}
	if payload.NVMeSmartHealth.PercentageUsed > 0 || payload.NVMeSmartHealth.MediaErrors > 0 {
		report.Attributes = append(report.Attributes,
			model.SmartAttribute{Name: "Percentage used", Raw: fmt.Sprintf("%d%%", payload.NVMeSmartHealth.PercentageUsed)},
			model.SmartAttribute{Name: "Media errors", Raw: strconv.Itoa(payload.NVMeSmartHealth.MediaErrors)},
		)
	}
	for _, message := range payload.Messages {
		if strings.Contains(strings.ToLower(message.String), "unavailable") {
			report.Message = message.String
			break
		}
	}
	return report, nil
}

// RunSmartTest starts the short offline self-test run by the Unraid disk page.
func (l *Live) RunSmartTest(ctx context.Context, device string) error {
	device = strings.TrimSpace(device)
	if !deviceNamePattern.MatchString(device) {
		return errors.New("invalid disk device")
	}
	binary := smartctlBinary()
	if binary == "" {
		return errors.New("smartctl is not installed on this host")
	}
	commandCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(commandCtx, binary, "-t", "short", "/dev/"+device).CombinedOutput(); err != nil {
		return fmt.Errorf("start SMART test: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}
