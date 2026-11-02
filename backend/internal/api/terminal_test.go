package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShellFromPasswd(t *testing.T) {
	shell := filepath.Join(t.TempDir(), "customsh")
	if err := os.WriteFile(shell, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "root:x:0:0:root:/root:/bin/sh\n" +
		"alice:x:1000:1000::/home/alice:" + shell + "\n" +
		"broken:x:1001:1001::/home/broken:/does/not/exist\n"

	if got := shellFromPasswd(content, "alice"); got != shell {
		t.Fatalf("alice shell = %q, want %q", got, shell)
	}
	if got := shellFromPasswd(content, "root"); got != "/bin/sh" {
		t.Fatalf("root shell = %q, want /bin/sh", got)
	}
	if got := shellFromPasswd(content, "broken"); got != "" {
		t.Fatalf("broken shell = %q, want empty fallback", got)
	}
	if got := shellFromPasswd(content, "missing"); got != "" {
		t.Fatalf("missing shell = %q, want empty fallback", got)
	}
}

func TestShellFromPasswdRejectsLoginDisablers(t *testing.T) {
	for _, shell := range []string{"/sbin/nologin", "/bin/false", "/usr/sbin/nologin"} {
		content := "alice:x:1000:1000::/home/alice:" + shell + "\n"
		if got := shellFromPasswd(content, "alice"); got != "" {
			t.Fatalf("%s accepted as login shell: %q", shell, got)
		}
	}
}

func TestLoginShellArgs(t *testing.T) {
	for _, shell := range []string{"/bin/bash", "/bin/zsh", "/usr/bin/fish", "/bin/sh"} {
		args := loginShellArgs(shell)
		if len(args) != 1 || args[0] != "-l" {
			t.Fatalf("%s args = %v, want -l", shell, args)
		}
	}
	if args := loginShellArgs("/usr/local/bin/custom-shell"); len(args) != 0 {
		t.Fatalf("custom shell args = %v, want none", args)
	}
}
