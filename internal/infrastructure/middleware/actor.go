package middleware

import (
	"net/http"

	"OzonBankTestTask/internal/requestctx"
)

// DemoActor передаёт X-User-ID в контекст для демонстрационного запуска.
// Идентичность из доверенного middleware имеет приоритет, в том числе пустая.
// Заголовок не подтверждает личность; отсутствие actor проверяют мутации.
func DemoActor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if !requestctx.HasActor(ctx) {
			ctx = requestctx.WithActor(ctx, r.Header.Get("X-User-ID"))
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
