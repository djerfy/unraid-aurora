// Package webui serves the embedded single-page application.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var embedded embed.FS

// Handler returns an http.Handler that serves the built SPA, falling back to
// index.html for client-side routes. Missing asset files return 404 instead of
// the HTML shell so broken imports are obvious.
func Handler() (http.Handler, error) {
	dist, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil, err
	}
	files := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean("/" + r.URL.Path)
		name := strings.TrimPrefix(clean, "/")
		if name != "" {
			if info, err := fs.Stat(dist, name); err == nil && !info.IsDir() {
				if strings.Contains(clean, "/assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
			if path.Ext(clean) != "" {
				http.NotFound(w, r)
				return
			}
		}
		serveIndex(dist, w, r)
	}), nil
}

func serveIndex(dist fs.FS, w http.ResponseWriter, r *http.Request) {
	raw, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		http.Error(w, "UI build is missing: run `make build`", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(raw)
}
