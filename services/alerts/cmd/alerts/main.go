// Package main is the entrypoint for the vigil-alerts service.
package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/uwezukwechibuzor/vigil/services/internal/platform"
)

func main() {
	platform.StartService("alerts", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"service":"vigil-alerts","status":"ok"}`))
		})
	})
}
