package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"OzonBankTestTask/internal/requestctx"
)

// newHTTPHandler проверяет обычные HTTP-запросы перед исполнением GraphQL.
func newHTTPHandler(next http.Handler, maxBytes int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := requestctx.RequestID(r.Context())
		switch r.Method {
		case http.MethodPost:
			if !validatePost(w, r, maxBytes) {
				return
			}
		case http.MethodGet:
			if !validateQueryLength(w, r, maxBytes) {
				return
			}
		case http.MethodOptions:
		default:
			w.Header().Set("Allow", "GET, POST, OPTIONS")
			writeHTTPError(w, id, http.StatusMethodNotAllowed, "BAD_USER_INPUT", "метод не поддерживается")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func validatePost(w http.ResponseWriter, r *http.Request, maxBytes int64) bool {
	id := requestctx.RequestID(r.Context())
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeHTTPError(w, id, http.StatusUnsupportedMediaType, "BAD_USER_INPUT", "ожидается Content-Type: application/json")
		return false
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBytes))
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			writeHTTPError(w, id, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "превышен размер запроса")
		} else {
			writeHTTPError(w, id, http.StatusBadRequest, "BAD_USER_INPUT", "не удалось прочитать запрос")
		}
		return false
	}
	trimmed := bytes.TrimSpace(body)
	if !json.Valid(body) || len(trimmed) == 0 || trimmed[0] != '{' {
		writeHTTPError(w, id, http.StatusBadRequest, "BAD_USER_INPUT", "ожидается один JSON-объект GraphQL-запроса")
		return false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return true
}

func validateQueryLength(w http.ResponseWriter, r *http.Request, maxBytes int64) bool {
	if int64(len(r.URL.RawQuery)) <= maxBytes {
		return true
	}
	writeHTTPError(w, requestctx.RequestID(r.Context()), http.StatusRequestURITooLong, "PAYLOAD_TOO_LARGE", "превышен размер запроса")
	return false
}
