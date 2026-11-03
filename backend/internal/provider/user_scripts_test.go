package provider

import "testing"

func TestUserScriptPathRejectsTraversal(t *testing.T) {
	for _, id := range []string{"../etc", "a/b", "", "..", "script name"} {
		if _, err := userScriptPath(id); err == nil {
			t.Errorf("userScriptPath(%q) was accepted", id)
		}
	}
	if _, err := userScriptPath("backup-daily"); err != nil {
		t.Errorf("valid script id rejected: %v", err)
	}
}
