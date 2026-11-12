package middleware

import (
	"net/http"

	"OzonBankTestTask/internal/requestctx"

	"github.com/google/uuid"
)

// RequestID создаёт серверный ID запроса и передаёт его в контекст и заголовок ответа.
// Входящий X-Request-ID не используется как источник идентификатора.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewString()
		w.Header().Set("X-Request-ID", id)
		ctx := requestctx.WithRequestID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
