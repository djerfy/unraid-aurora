package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/coder/websocket"
)

// vmConsole proxies the libvirt VNC/SPICE console. The target port comes from
// the domain XML, never from the client, so the endpoint cannot be used to
// reach arbitrary localhost services.
//
// noVNC tunnels the raw RFB stream over WebSocket. When the domain exposes a
// TCP VNC port we bridge straight to it (the websockify model); otherwise we
// fall back to QEMU's own websocket listener.
func (s *Server) vmConsole(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" {
		originURL, err := url.Parse(origin)
		if err != nil || !sameHost(originURL.Host, r.Host) {
			writeError(w, http.StatusForbidden, "origin", "cross-origin request rejected")
			return
		}
	}

	resolveCtx, cancelResolve := context.WithTimeout(r.Context(), 10*time.Second)
	console, err := s.provider.VMConsole(resolveCtx, r.PathValue("id"))
	cancelResolve()
	if err != nil {
		writeError(w, http.StatusConflict, "console_unavailable", err.Error())
		return
	}

	client, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // Origin checked above.
		CompressionMode:    websocket.CompressionDisabled,
	})
	if err != nil {
		return
	}
	defer client.Close(websocket.StatusNormalClosure, "console closed")
	client.SetReadLimit(16 << 20)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	if console.Port > 0 && console.Port <= 65535 {
		address := net.JoinHostPort("127.0.0.1", strconv.Itoa(console.Port))
		upstream, err := net.DialTimeout("tcp", address, 5*time.Second)
		if err == nil {
			defer upstream.Close()
			relayTCPConsole(ctx, client, upstream)
			return
		}
		if console.WebSocket < 1 {
			_ = client.Write(ctx, websocket.MessageText, []byte("Unable to reach the virtual machine console: "+err.Error()))
			return
		}
	}

	if console.WebSocket > 0 {
		upstreamURL := fmt.Sprintf("ws://127.0.0.1:%d/", console.WebSocket)
		upstream, _, err := websocket.Dial(ctx, upstreamURL, &websocket.DialOptions{
			CompressionMode: websocket.CompressionDisabled,
		})
		if err != nil {
			_ = client.Write(ctx, websocket.MessageText, []byte("Unable to reach the virtual machine console: "+err.Error()))
			return
		}
		defer upstream.Close(websocket.StatusNormalClosure, "console closed")
		upstream.SetReadLimit(64 << 20)
		relayWebSocket(ctx, client, upstream)
		return
	}

	_ = client.Write(ctx, websocket.MessageText, []byte("This virtual machine does not expose a console."))
}

// relayTCPConsole bridges binary WebSocket frames to the raw RFB stream.
func relayTCPConsole(ctx context.Context, client *websocket.Conn, upstream net.Conn) {
	errCh := make(chan error, 2)

	go func() {
		for {
			_, data, err := client.Read(ctx)
			if err != nil {
				errCh <- err
				return
			}
			if _, err := upstream.Write(data); err != nil {
				errCh <- err
				return
			}
		}
	}()

	go func() {
		buffer := make([]byte, 32*1024)
		for {
			read, err := upstream.Read(buffer)
			if read > 0 {
				if writeErr := client.Write(ctx, websocket.MessageBinary, buffer[:read]); writeErr != nil {
					errCh <- writeErr
					return
				}
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					errCh <- err
				}
				return
			}
		}
	}()

	<-errCh
}

func relayWebSocket(ctx context.Context, client, upstream *websocket.Conn) error {
	errCh := make(chan error, 2)
	go func() {
		for {
			messageType, data, err := client.Read(ctx)
			if err != nil {
				errCh <- err
				return
			}
			if err := upstream.Write(ctx, messageType, data); err != nil {
				errCh <- err
				return
			}
		}
	}()
	go func() {
		for {
			messageType, data, err := upstream.Read(ctx)
			if err != nil {
				errCh <- err
				return
			}
			if err := client.Write(ctx, messageType, data); err != nil {
				errCh <- err
				return
			}
		}
	}()
	return <-errCh
}
