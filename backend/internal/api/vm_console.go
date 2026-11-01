package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/coder/websocket"
)

// vmConsole proxies the libvirt VNC/SPICE websocket. The upstream port comes
// from the domain XML, never from the client, so the endpoint cannot be used to
// reach arbitrary localhost services.
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

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	upstreamURL := fmt.Sprintf("ws://127.0.0.1:%d/", console.WebSocket)
	upstream, _, err := websocket.Dial(ctx, upstreamURL, &websocket.DialOptions{
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		_ = client.Write(ctx, websocket.MessageText, []byte("Unable to reach the virtual machine console: "+err.Error()))
		return
	}
	defer upstream.Close(websocket.StatusNormalClosure, "console closed")

	// Framebuffer updates are much larger than the default 32 KiB read limit.
	client.SetReadLimit(16 << 20)
	upstream.SetReadLimit(64 << 20)

	errors := make(chan error, 2)
	go func() { errors <- relayWebSocket(ctx, upstream, client) }()
	go func() { errors <- relayWebSocket(ctx, client, upstream) }()
	<-errors
	cancel()
}

func relayWebSocket(ctx context.Context, destination, source *websocket.Conn) error {
	for {
		messageType, data, err := source.Read(ctx)
		if err != nil {
			return err
		}
		if err := destination.Write(ctx, messageType, data); err != nil {
			return err
		}
	}
}
