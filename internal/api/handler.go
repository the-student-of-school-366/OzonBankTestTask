package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"

	graph "OzonBankTestTask/internal/graphql"
	"OzonBankTestTask/internal/graphql/limits"
	"OzonBankTestTask/internal/infrastructure/middleware"
	"OzonBankTestTask/internal/requestctx"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// NewHandler создаёт обработчик GraphQL, который можно смонтировать на /graphql.
// X-User-ID — демонстрационный способ передачи actor ID, не аутентификация.
// Доверенный middleware может установить actor через WithActor до вызова обработчика.
func NewHandler(posts PostUseCase, comments CommentUseCase, config Config) (http.Handler, error) {
	if posts == nil || comments == nil {
		return nil, fmt.Errorf("api: post and comment usecases are required")
	}
	config, err := config.withDefaults()
	if err != nil {
		return nil, err
	}

	server := graph.NewServer(posts, comments, limits.Config{
		MaxDepth: config.MaxDepth, MaxComplexity: config.MaxComplexity,
	}, config.Logger)

	serve := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := requestctx.RequestID(r.Context())
		switch r.Method {
		case http.MethodPost:
			mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || mediaType != "application/json" {
				writeHTTPError(w, id, http.StatusUnsupportedMediaType, "BAD_USER_INPUT", "ожидается Content-Type: application/json")
				return
			}
			body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, config.MaxBodyBytes))
			if err != nil {
				var limit *http.MaxBytesError
				if errors.As(err, &limit) {
					writeHTTPError(w, id, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "превышен размер запроса")
				} else {
					writeHTTPError(w, id, http.StatusBadRequest, "BAD_USER_INPUT", "не удалось прочитать запрос")
				}
				return
			}
			trimmed := bytes.TrimSpace(body)
			if !json.Valid(body) || len(trimmed) == 0 || trimmed[0] != '{' {
				writeHTTPError(w, id, http.StatusBadRequest, "BAD_USER_INPUT", "ожидается один JSON-объект GraphQL-запроса")
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
		case http.MethodGet:
			if int64(len(r.URL.RawQuery)) > config.MaxBodyBytes {
				writeHTTPError(w, id, http.StatusRequestURITooLong, "PAYLOAD_TOO_LARGE", "превышен размер запроса")
				return
			}
		case http.MethodOptions:
		default:
			w.Header().Set("Allow", "GET, POST, OPTIONS")
			writeHTTPError(w, id, http.StatusMethodNotAllowed, "BAD_USER_INPUT", "метод не поддерживается")
			return
		}
		server.ServeHTTP(w, r)
	})
	handler := middleware.Timeout(config.RequestTimeout, func(r *http.Request) string {
		body, _ := json.Marshal(&graphql.Response{Errors: gqlerror.List{{
			Message:    "превышено время выполнения запроса",
			Extensions: map[string]any{"code": "TIMEOUT", "requestId": requestctx.RequestID(r.Context())},
		}}})
		return string(body)
	})(serve)
	handler = middleware.DemoActor(handler)
	handler = middleware.RequestID(handler)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		handler.ServeHTTP(w, r)
	}), nil
}

func writeHTTPError(w http.ResponseWriter, id string, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(&graphql.Response{Errors: gqlerror.List{{
		Message: message, Extensions: map[string]any{"code": code, "requestId": id},
	}}})
}
