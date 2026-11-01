package provider

import "testing"

func TestValidateUserInputs(t *testing.T) {
	if err := validateUserName("jellyfin"); err != nil {
		t.Fatalf("valid name rejected: %v", err)
	}
	for _, name := range []string{"Root", "user name", "1user", ""} {
		if err := validateUserName(name); err == nil {
			t.Fatalf("%q should be rejected", name)
		}
	}
	if err := validateUserPassword("correct horse battery staple"); err != nil {
		t.Fatalf("password rejected: %v", err)
	}
	if err := validateUserPassword(""); err == nil {
		t.Fatal("empty password should be rejected")
	}
	if err := validateUserDescription("Media account"); err != nil {
		t.Fatalf("description rejected: %v", err)
	}
	if err := validateUserDescription("bad:value"); err == nil {
		t.Fatal("colon should be rejected")
	}
}

func TestEncodeUserPassword(t *testing.T) {
	if got := encodeUserPassword("secret"); got != "c2VjcmV0" {
		t.Fatalf("base64 = %q", got)
	}
}
