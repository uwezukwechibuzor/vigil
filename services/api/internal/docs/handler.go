// Package docs serves the OpenAPI spec and Redoc documentation UI.
package docs

import (
	_ "embed"
	"net/http"
)

//go:embed redoc.html
var redocHTML []byte

// Handler returns an http.Handler that serves:
//   - GET /docs    — Redoc UI
//   - GET /openapi.yaml — raw OpenAPI 3.1 spec
func Handler(spec []byte) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(redocHTML)
	})
	mux.HandleFunc("/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(spec)
	})
	return mux
}
