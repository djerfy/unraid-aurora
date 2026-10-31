package provider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseINI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "var.ini")
	content := `; comment
NAME="Vault"
mdState="STARTED"
mdNumDisks='6'

[disk1]
device="sdc"
fsType="xfs"
fsSize="17578125000"
temp="35"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	ini, err := parseINI(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := ini.get("", "NAME"); got != "Vault" {
		t.Fatalf("NAME = %q", got)
	}
	if got := ini.get("", "mdNumDisks"); got != "6" {
		t.Fatalf("mdNumDisks = %q", got)
	}
	if got := ini.get("disk1", "device"); got != "sdc" {
		t.Fatalf("disk1.device = %q", got)
	}
	if got := parseKiB(ini.get("disk1", "fsSize")); got != 17578125000*1024 {
		t.Fatalf("fsSize = %d", got)
	}
}

func TestParseINIMissingFile(t *testing.T) {
	if _, err := parseINI(filepath.Join(t.TempDir(), "missing.ini")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestParseKiB(t *testing.T) {
	cases := map[string]uint64{"0": 0, "1024": 1024 * 1024, "": 0, "bogus": 0, "-12": 0}
	for input, want := range cases {
		if got := parseKiB(input); got != want {
			t.Fatalf("parseKiB(%q) = %d, want %d", input, got, want)
		}
	}
}
