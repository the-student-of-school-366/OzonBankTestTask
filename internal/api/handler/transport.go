package handler

import (
	"net/http"
	"strings"

	"OzonBankTestTask/internal/requestctx"
)

// routeTransport пропускает WebSocket мимо HTTP-таймаута, сохраняя ограничения upgrade.
func routeTransport(websocket, ordinary http.Handler, maxBytes int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") == "" {
			ordinary.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet || !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			writeHTTPError(w, requestctx.RequestID(r.Context()), http.StatusBadRequest, "BAD_USER_INPUT", "некорректный WebSocket upgrade")
			return
		}
		if !validateQueryLength(w, r, maxBytes) {
			return
		}
		websocket.ServeHTTP(w, r)
	})
}

func responseHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTP(w, r)
	})
}
