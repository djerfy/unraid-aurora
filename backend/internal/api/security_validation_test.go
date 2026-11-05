package api

import "testing"

func TestContainerIDPatternRejectsExecFlagInjection(t *testing.T) {
	for _, value := range []string{"abc123", "plex", "container_1", "a.b-c"} {
		if !containerIDPattern.MatchString(value) {
			t.Errorf("%q should be accepted", value)
		}
	}
	for _, value := range []string{"", "-rf", "--privileged", "a/b", "a b", "a\x00b"} {
		if containerIDPattern.MatchString(value) {
			t.Errorf("%q must be rejected", value)
		}
	}
}
