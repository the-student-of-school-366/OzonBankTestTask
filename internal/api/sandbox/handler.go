// Package sandbox предоставляет страницу Apollo Sandbox для разработки.
package sandbox

import (
	_ "embed"
	"io"
	"net/http"
)

//go:embed index.html
var page string

func NewHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, page)
		}
	})
}
