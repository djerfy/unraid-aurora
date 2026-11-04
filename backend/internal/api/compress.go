package api

import (
	"compress/gzip"
	"net/http"
	"strings"
)

type gzipResponseWriter struct {
	http.ResponseWriter
	writer      *gzip.Writer
	wroteHeader bool
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.Header().Del("Content-Length")
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Add("Vary", "Accept-Encoding")
	w.ResponseWriter.WriteHeader(status)
}

func (w *gzipResponseWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.writer.Write(data)
}

func (w *gzipResponseWriter) Flush() {
	_ = w.writer.Flush()
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// compress gzips JSON reads. Streaming responses (SSE, NDJSON updates and
// consoles) are left untouched so their backpressure and framing stay intact.
func (s *Server) compress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("Upgrade") != "" ||
			strings.Contains(r.Header.Get("Accept"), "text/event-stream") ||
			!compressiblePath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		defer gz.Close()
		next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, writer: gz}, r)
	})
}

func compressiblePath(path string) bool {
	if path == "/api/openapi.json" || path == "/api/docs" {
		return true
	}
	if !strings.HasPrefix(path, "/api/") {
		return false
	}
	for _, suffix := range []string{"/events", "/terminal", "/console", "/update", "/install", "/logs"} {
		if strings.HasSuffix(path, suffix) {
			return false
		}
	}
	return true
}
