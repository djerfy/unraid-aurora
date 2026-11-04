package main

import "testing"

func TestDisplayVersion(t *testing.T) {
	previous := version
	defer func() { version = previous }()
	for input, want := range map[string]string{
		"0.0.8":     "v0.0.8",
		"v0.0.8":    "v0.0.8",
		"0.0.0-dev": "v0.0.0-dev",
	} {
		version = input
		if got := displayVersion(); got != want {
			t.Errorf("displayVersion(%q) = %q, want %q", input, got, want)
		}
	}
}
