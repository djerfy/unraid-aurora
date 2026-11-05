package provider

import "testing"

func TestSecurityIdentifiersRejectArgumentAndPathInjection(t *testing.T) {
	cases := []struct {
		name    string
		valid   []string
		invalid []string
		check   func(string) bool
	}{
		{
			name:    "vm names",
			valid:   []string{"plex", "Windows 11", "ubuntu-ci", "vm.01"},
			invalid: []string{"", "-rf", "../etc", "a/b", "a\x00b"},
			check:   func(value string) bool { return vmNamePattern.MatchString(value) && !containsDotDot(value) },
		},
		{
			name:    "share names",
			valid:   []string{"media", "appdata", "backup_01"},
			invalid: []string{"", "-rf", "../media", "a/b", "1media"},
			check:   func(value string) bool { return shareNamePattern.MatchString(value) },
		},
		{
			name:    "user script ids",
			valid:   []string{"backup", "nightly.01", "script_1"},
			invalid: []string{"", "-rf", "../script", "a/b", ".hidden"},
			check:   func(value string) bool { return userScriptIDPattern.MatchString(value) },
		},
		{
			name:    "zfs datasets",
			valid:   []string{"tank", "tank/data", "tank/data/child"},
			invalid: []string{"", "-rf", "../tank", "tank/../etc", "tank data"},
			check:   func(value string) bool { return zfsDatasetPattern.MatchString(value) && !containsDotDot(value) },
		},
		{
			name:    "zfs snapshot names",
			valid:   []string{"snap", "before-update", "daily.01"},
			invalid: []string{"", "-rf", "../snap", "a/b", "snap shot"},
			check:   func(value string) bool { return zfsNamePattern.MatchString(value) },
		},
	}
	for _, tc := range cases {
		for _, value := range tc.valid {
			if !tc.check(value) {
				t.Errorf("%s: %q should be accepted", tc.name, value)
			}
		}
		for _, value := range tc.invalid {
			if tc.check(value) {
				t.Errorf("%s: %q must be rejected", tc.name, value)
			}
		}
	}
}

func containsDotDot(value string) bool {
	for i := 0; i+1 < len(value); i++ {
		if value[i] == '.' && value[i+1] == '.' {
			return true
		}
	}
	return false
}

func TestConfigPathOnlyAcceptsBaseNames(t *testing.T) {
	live := &Live{configRoot: t.TempDir()}
	if got := live.configPath("settings.json"); got == "" {
		t.Fatal("a base name should be accepted")
	}
	for _, value := range []string{"", "..", "../settings.json", "sub/settings.json", "/etc/passwd"} {
		if got := live.configPath(value); got != "" {
			t.Errorf("configPath(%q) = %q, want empty", value, got)
		}
	}
}
