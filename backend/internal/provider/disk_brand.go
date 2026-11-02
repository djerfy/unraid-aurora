package provider

import "strings"

var diskBrands = []string{
	"Western Digital", "Seagate", "Toshiba", "Samsung", "HGST", "Hitachi",
	"Micron", "Crucial", "Kingston", "SanDisk", "SK hynix", "Kioxia",
	"Plextor", "Corsair", "Sabrent", "Intel", "WDC",
}

// diskBrand extracts a vendor from the model string exposed by Unraid. Models
// are either "Brand Model" or a compact vendor prefix such as WDC/ST/WD.
func diskBrand(model string) string {
	value := strings.TrimSpace(model)
	upper := strings.ToUpper(value)
	for _, brand := range diskBrands {
		prefix := strings.ToUpper(brand)
		if upper == prefix || strings.HasPrefix(upper, prefix+" ") || strings.HasPrefix(upper, prefix+"_") {
			return brand
		}
	}
	if strings.HasPrefix(upper, "WDC") {
		return "WDC"
	}
	if strings.HasPrefix(upper, "HGST") {
		return "HGST"
	}
	if len(value) > 2 && strings.HasPrefix(upper, "WD") && upper[2] >= '0' && upper[2] <= '9' {
		return "WD"
	}
	if len(value) > 2 && strings.HasPrefix(upper, "ST") && upper[2] >= '0' && upper[2] <= '9' {
		return "Seagate"
	}
	return ""
}
