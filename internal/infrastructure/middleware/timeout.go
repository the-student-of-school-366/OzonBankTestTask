package middleware

import (
	"net/http"
	"time"
)

// Timeout ограничивает время HTTP-ответа и отменяет контекст обработчика.
// Даже если обработчик игнорирует контекст, клиент получает HTTP 503 по истечении duration.
// responseBody формирует тело ошибки из метаданных запроса; nil включает стандартный ответ net/http.
// Content-Type для собственного тела нужно установить до вызова middleware.
// Как и http.TimeoutHandler, middleware не поддерживает streaming и WebSocket.
func Timeout(duration time.Duration, responseBody func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body string
			if responseBody != nil {
				body = responseBody(r)
			}
			http.TimeoutHandler(next, duration, body).ServeHTTP(w, r)
		})
	}
}
