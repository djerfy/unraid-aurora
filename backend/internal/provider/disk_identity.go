package provider

import (
	"os"
	"path/filepath"
	"strings"
)

// diskIdentity uses Unraid's id/idSb value, which is model_serial. deviceSb is
// a kernel mapping (md1, sdi1), not the physical model.
func diskIdentity(entry map[string]string) (brand, model, serial string) {
	id := strings.TrimSpace(entry["id"])
	if id == "" {
		id = strings.TrimSpace(entry["idSb"])
	}
	if id != "" {
		modelPart := id
		if index := strings.LastIndex(id, "_"); index > 0 {
			prefix := id[:index]
			// A single underscore can separate a brand from its model
			// (WDC_WD140EDGZ) instead of model from serial.
			if strings.Count(id, "_") > 1 || !isBrandToken(prefix) {
				modelPart = prefix
				serial = id[index+1:]
			}
		}
		brand = diskBrand(modelPart)
		model = strings.ReplaceAll(stripDiskBrand(modelPart, brand), "_", " ")
	}
	if serial == "" {
		serial = strings.TrimSpace(entry["serial"])
	}
	if model == "" || serial == "" {
		sysBrand, sysModel, sysSerial := diskSysfs(entry["device"])
		if brand == "" {
			brand = sysBrand
		}
		if model == "" {
			model = sysModel
		}
		if serial == "" {
			serial = sysSerial
		}
	}
	return brand, model, serial
}

func isBrandToken(value string) bool {
	upper := strings.ToUpper(strings.TrimSpace(value))
	for _, brand := range []string{
		"WESTERN DIGITAL", "SEAGATE", "TOSHIBA", "SAMSUNG", "HGST", "HITACHI",
		"MICRON", "CRUCIAL", "KINGSTON", "SANDISK", "SK HYNIX", "KIOXIA",
		"PLEXTOR", "CORSAIR", "SABRENT", "INTEL", "WDC", "WD",
	} {
		if upper == brand {
			return true
		}
	}
	return false
}

func stripDiskBrand(model, brand string) string {
	if brand == "" || len(model) <= len(brand) || !strings.EqualFold(model[:len(brand)], brand) {
		return model
	}
	separator := model[len(brand)]
	if separator != '_' && separator != ' ' {
		return model
	}
	return model[len(brand)+1:]
}

func diskSysfs(device string) (brand, model, serial string) {
	if device == "" {
		return "", "", ""
	}
	base := filepath.Join("/sys/block", device, "device")
	model = readSysfsValue(filepath.Join(base, "model"))
	vendor := readSysfsValue(filepath.Join(base, "vendor"))
	serial = readSysfsValue(filepath.Join(base, "serial"))
	if serial == "" {
		serial = readSysfsValue(filepath.Join("/sys/block", device, "serial"))
	}
	brand = diskBrand(vendor + " " + model)
	if brand == "" {
		brand = strings.TrimSpace(vendor)
	}
	return brand, model, serial
}

func readSysfsValue(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}
