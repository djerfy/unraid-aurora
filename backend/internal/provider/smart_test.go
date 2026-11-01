package provider

import "testing"

const ataSmartJSON = `{
  "model_name": "WDC WD40EFRX",
  "serial_number": "WD-123",
  "firmware_version": "82.00A82",
  "smart_status": {"passed": true},
  "temperature": {"current": 34},
  "power_on_time": {"hours": 12345},
  "ata_smart_attributes": {
    "table": [
      {"id": 5, "name": "Reallocated_Sector_Ct", "value": 200, "worst": 200, "thresh": 140, "when_failed": "", "raw": {"value": 0, "string": "0"}},
      {"id": 197, "name": "Current_Pending_Sector", "value": 1, "worst": 1, "thresh": 0, "when_failed": "now", "raw": {"value": 8, "string": "8"}}
    ]
  },
  "ata_smart_self_test_log": {"standard": [{"status": {"string": "Completed without error", "passed": true}}]}
}`

func TestParseSmartctlPayloadATA(t *testing.T) {
	report, err := parseSmartctlPayload([]byte(ataSmartJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !report.Available || !report.Passed {
		t.Fatalf("availability/health = %+v", report)
	}
	if report.Model != "WDC WD40EFRX" || report.Serial != "WD-123" || report.Firmware != "82.00A82" {
		t.Fatalf("identity = %+v", report)
	}
	if report.Temperature != 34 || report.PowerOnHours != 12345 {
		t.Fatalf("metrics = %+v", report)
	}
	if report.SelfTest != "Completed without error" {
		t.Fatalf("self test = %q", report.SelfTest)
	}
	if len(report.Attributes) != 2 || !report.Attributes[1].Failing || report.Attributes[1].Raw != "8" {
		t.Fatalf("attributes = %+v", report.Attributes)
	}
}

func TestParseSmartctlPayloadNVMe(t *testing.T) {
	raw := `{
  "model_name": "Samsung SSD 990 PRO",
  "smart_status": {"passed": true},
  "nvme_smart_health_information_log": {
    "temperature": 311,
    "power_on_hours": 987,
    "percentage_used": 3,
    "media_errors": 2,
    "critical_warning": 0
  }
}`
	report, err := parseSmartctlPayload([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if report.Temperature != 38 {
		t.Fatalf("kelvin conversion = %d", report.Temperature)
	}
	if report.PowerOnHours != 987 {
		t.Fatalf("power on hours = %d", report.PowerOnHours)
	}
	if len(report.Attributes) != 2 {
		t.Fatalf("nvme pseudo attributes = %+v", report.Attributes)
	}
}
