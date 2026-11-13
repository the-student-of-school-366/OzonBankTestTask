package handler

import (
	"encoding/json"
	"net/http"

	"OzonBankTestTask/internal/requestctx"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func timeoutBody(r *http.Request) string {
	body, _ := json.Marshal(&graphql.Response{Errors: gqlerror.List{{
		Message:    "превышено время выполнения запроса",
		Extensions: map[string]any{"code": "TIMEOUT", "requestId": requestctx.RequestID(r.Context())},
	}}})
	return string(body)
}

func writeHTTPError(w http.ResponseWriter, id string, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(&graphql.Response{Errors: gqlerror.List{{
		Message: message, Extensions: map[string]any{"code": code, "requestId": id},
	}}})
}
