package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/coder/websocket"
	"github.com/creack/pty"
)

var containerIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)

type terminalResize struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// terminal starts an interactive root shell over WebSocket. The request has
// already passed cookie authentication; Origin is verified here because the
// standard CSRF middleware does not apply to GET upgrades.
func (s *Server) terminal(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" {
		originURL, err := url.Parse(origin)
		if err != nil || !sameHost(originURL.Host, r.Host) {
			writeError(w, http.StatusForbidden, "origin", "cross-origin request rejected")
			return
		}
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // Origin checked above.
		CompressionMode:    websocket.CompressionDisabled,
	})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "terminal closed")

	username := "root"
	if sess, ok := s.sessionUser(r); ok {
		username = sess.User
	}
	shell := loginShell(username)
	container := strings.TrimSpace(r.URL.Query().Get("container"))
	var cmd *exec.Cmd
	if container != "" {
		if !containerIDPattern.MatchString(container) {
			_ = conn.Write(r.Context(), websocket.MessageText, []byte("Invalid container id.\r\n"))
			return
		}
		cmd = exec.Command("docker", "exec", "-it", container, "/bin/sh")
	} else {
		cmd = exec.Command(shell, loginShellArgs(shell)...)
	}
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "LANG=C.UTF-8")
	cmd.Env = withEnvValue(cmd.Env, "SHELL", shell)
	terminal, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 32, Cols: 120})
	if err != nil {
		_ = conn.Write(r.Context(), websocket.MessageText, []byte("Unable to start shell: "+err.Error()+"\r\n"))
		return
	}
	defer func() {
		_ = terminal.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		buffer := make([]byte, 8192)
		for {
			n, err := terminal.Read(buffer)
			if n > 0 {
				if writeErr := conn.Write(ctx, websocket.MessageBinary, buffer[:n]); writeErr != nil {
					cancel()
					return
				}
			}
			if err != nil {
				cancel()
				return
			}
		}
	}()

	for {
		messageType, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		switch messageType {
		case websocket.MessageText:
			var resize terminalResize
			if json.Unmarshal(data, &resize) == nil && resize.Type == "resize" && resize.Cols > 0 && resize.Rows > 0 {
				_ = pty.Setsize(terminal, &pty.Winsize{Cols: resize.Cols, Rows: resize.Rows})
			}
		case websocket.MessageBinary:
			if _, err := terminal.Write(data); err != nil {
				return
			}
		}
	}
}

// withEnvValue replaces an existing variable so the PTY does not inherit a
// stale SHELL from the Aurora service.
func withEnvValue(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			out = append(out, entry)
		}
	}
	return append(out, prefix+value)
}

// loginShell returns the account's shell from /etc/passwd, then $SHELL and
// /bin/sh as fallbacks. Aurora authenticates against the host, so the terminal
// should match what the same user gets over SSH.
func loginShell(username string) string {
	if data, err := os.ReadFile("/etc/passwd"); err == nil {
		if shell := shellFromPasswd(string(data), username); shell != "" {
			return shell
		}
	}
	if shell := os.Getenv("SHELL"); usableShell(shell) {
		return shell
	}
	return "/bin/sh"
}

func shellFromPasswd(content, username string) string {
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Split(line, ":")
		if len(fields) < 7 || fields[0] != username {
			continue
		}
		shell := fields[6]
		if usableShell(shell) {
			return shell
		}
	}
	return ""
}

func usableShell(path string) bool {
	if path == "" || !filepath.IsAbs(path) {
		return false
	}
	switch filepath.Base(path) {
	case "nologin", "false", "sync", "shutdown", "halt":
		return false
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
		return false
	}
	return true
}

// Not every shell understands -l; known POSIX shells use it to load the same
// login profile as an SSH session.
func loginShellArgs(shell string) []string {
	switch filepath.Base(shell) {
	case "bash", "zsh", "sh", "dash", "ksh", "mksh", "ash", "fish", "tcsh", "csh":
		return []string{"-l"}
	default:
		return nil
	}
}
