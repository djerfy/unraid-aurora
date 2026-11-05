package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// unraidVersionFile is a variable so tests can point at a temporary file.
var unraidVersionFile = "/etc/unraid-version"

var (
	unraidVersionAssignment = regexp.MustCompile(`(?im)^\s*version\s*=\s*"?([0-9][^"\r\n\t ]*)`)
	unraidVersionToken      = regexp.MustCompile(`^[vV]?([0-9]+)(?:\.[0-9]+)?`)
)

// DetectUnraidVersion reads the host version from /etc/unraid-version, with
// the emhttp var.ini state file as a fallback.
func DetectUnraidVersion(stateDir string) (string, error) {
	if raw, err := os.ReadFile(unraidVersionFile); err == nil {
		if version := parseUnraidVersion(string(raw)); version != "" {
			return version, nil
		}
	}
	if stateDir != "" {
		if raw, err := os.ReadFile(filepath.Join(stateDir, "var.ini")); err == nil {
			if version := parseUnraidVersion(string(raw)); version != "" {
				return version, nil
			}
		}
	}
	return "", fmt.Errorf("cannot determine the Unraid version from %s or %s",
		unraidVersionFile, filepath.Join(stateDir, "var.ini"))
}

func parseUnraidVersion(raw string) string {
	if match := unraidVersionAssignment.FindStringSubmatch(raw); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	for _, line := range strings.Split(raw, "\n") {
		value := strings.Trim(strings.TrimSpace(line), `"'`)
		if unraidVersionToken.MatchString(value) {
			return value
		}
	}
	return ""
}

// RequireUnraid7 rejects every major version other than 7. Aurora reads state
// files and commands that are not guaranteed to be compatible elsewhere, so an
// unknown version is refused instead of risking a partial or destructive run.
func RequireUnraid7(version string) error {
	major, err := unraidMajor(version)
	if err != nil {
		return err
	}
	if major != 7 {
		return fmt.Errorf("Unraid %s is not supported: Aurora requires Unraid 7.x", version)
	}
	return nil
}

func unraidMajor(version string) (int, error) {
	value := strings.TrimSpace(version)
	value = strings.TrimPrefix(strings.TrimPrefix(value, "v"), "V")
	match := unraidVersionToken.FindStringSubmatch(value)
	if len(match) < 2 {
		return 0, fmt.Errorf("invalid Unraid version %q", version)
	}
	major, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, fmt.Errorf("invalid Unraid version %q: %w", version, err)
	}
	return major, nil
}
