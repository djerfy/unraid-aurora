package api

import (
	_ "embed"
	"net/http"
)

//go:embed apidocs/openapi.json
var openAPIDocument []byte

const apiDocsHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Aurora API reference</title>
  <style>
    html, body { margin: 0; padding: 0; background: #0b0c10; }
    #redoc { min-height: 100vh; }
    .aurora-loading {
      display: grid; min-height: 100vh; place-items: center;
      font: 14px ui-sans-serif, system-ui, sans-serif; color: #9aa1b2;
    }
  </style>
</head>
<body>
  <div id="redoc"><div class="aurora-loading">Loading the API reference…</div></div>
  <script type="module" src="/redoc/init.js"></script>
</body>
</html>
`

// apiDocs serves the read-only Redoc reference. The page itself is protected by
// the normal API authentication middleware; Redoc has no request playground.
func (s *Server) apiDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(apiDocsHTML))
}

func (s *Server) openAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(openAPIDocument)
}
