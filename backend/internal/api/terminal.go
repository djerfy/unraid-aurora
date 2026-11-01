package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/exec"

	"github.com/coder/websocket"
	"github.com/creack/pty"
)

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

	shell := "/bin/bash"
	cmd := exec.Command(shell, "-l")
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "LANG=C.UTF-8")
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
