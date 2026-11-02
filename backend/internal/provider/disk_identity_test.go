package provider

import "testing"

func TestDiskIdentity(t *testing.T) {
	cases := []struct {
		id     string
		brand  string
		model  string
		serial string
	}{
		{"ST18000NM000J-2TV103_ZR585CPY", "Seagate", "ST18000NM000J-2TV103", "ZR585CPY"},
		{"WDC_WD120EDAZ-11F3RA0_5PJRD45C", "WDC", "WD120EDAZ-11F3RA0", "5PJRD45C"},
		{"Samsung_SSD_850_EVO_250GB_S2R5NX0H643734Z", "Samsung", "SSD 850 EVO 250GB", "S2R5NX0H643734Z"},
		{"KINGSTON_SA2000M8250G_50026B7282669D9E", "Kingston", "SA2000M8250G", "50026B7282669D9E"},
	}
	for _, tc := range cases {
		brand, model, serial := diskIdentity(map[string]string{"id": tc.id})
		if brand != tc.brand || model != tc.model || serial != tc.serial {
			t.Errorf("diskIdentity(%q) = %q/%q/%q, want %q/%q/%q", tc.id, brand, model, serial, tc.brand, tc.model, tc.serial)
		}
	}
}

func TestDiskIdentityFallsBackToSerialField(t *testing.T) {
	brand, model, serial := diskIdentity(map[string]string{"id": "WDC_WD140EDGZ", "serial": "ABC123"})
	if brand != "WDC" || model != "WD140EDGZ" || serial != "ABC123" {
		t.Fatalf("identity = %q/%q/%q", brand, model, serial)
	}
}

func TestDiskMedia(t *testing.T) {
	cases := []struct {
		entry map[string]string
		want  string
	}{
		{map[string]string{"device": "sdh", "rotational": "1"}, "hdd"},
		{map[string]string{"device": "sdi", "rotational": "0"}, "ssd"},
		{map[string]string{"device": "nvme0n1", "rotational": "0"}, "nvme"},
		{map[string]string{"device": "sda", "transport": "nvme"}, "nvme"},
		{map[string]string{"device": "sda"}, ""},
	}
	for _, tc := range cases {
		if got := diskMedia(tc.entry); got != tc.want {
			t.Errorf("diskMedia(%v) = %q, want %q", tc.entry, got, tc.want)
		}
	}
}
