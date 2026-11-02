package provider

import "testing"

func TestDiskBrand(t *testing.T) {
	cases := map[string]string{
		"WDC WD180EDGZ-11":        "WDC",
		"WD140EDGZ":               "WD",
		"ST16000NM001G":           "Seagate",
		"Seagate Exos X18":        "Seagate",
		"Samsung SSD 990 PRO 2TB": "Samsung",
		"Toshiba MG08ACA16TE":     "Toshiba",
		"HGST HUH721010ALE600":    "HGST",
		"":                        "",
	}
	for model, want := range cases {
		if got := diskBrand(model); got != want {
			t.Errorf("diskBrand(%q) = %q, want %q", model, got, want)
		}
	}
}
