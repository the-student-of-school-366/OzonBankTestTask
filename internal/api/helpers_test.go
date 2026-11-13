package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"OzonBankTestTask/internal/api"
	"OzonBankTestTask/internal/api/handler"
	"OzonBankTestTask/internal/dto"
	"OzonBankTestTask/internal/graphql/mocks"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type graphResponse struct {
	Data   map[string]json.RawMessage `json:"data"`
	Errors []struct {
		Message    string         `json:"message"`
		Path       []any          `json:"path"`
		Extensions map[string]any `json:"extensions"`
	} `json:"errors"`
}

func newHandler(t *testing.T, config api.Config) (http.Handler, *mocks.PostUseCase, *mocks.CommentUseCase) {
	t.Helper()
	posts, comments := mocks.NewPostUseCase(t), mocks.NewCommentUseCase(t)
	config.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := handler.NewHandler(posts, comments, mocks.NewSubscriptionUseCase(t), config)
	require.NoError(t, err)
	t.Cleanup(h.Close)
	return h, posts, comments
}

func request(t *testing.T, h http.Handler, query string, variables map[string]any, actor string) (*httptest.ResponseRecorder, graphResponse) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	require.NoError(t, err)
	r := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-User-ID", actor)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var response graphResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response), w.Body.String())
	return w, response
}

func assertCode(t *testing.T, response graphResponse, code string) {
	t.Helper()
	require.NotEmpty(t, response.Errors)
	require.Equal(t, code, response.Errors[0].Extensions["code"], response.Errors)
}

func examplePost() dto.Post {
	return dto.Post{ID: uuid.NewString(), Title: "title", Content: "body", AuthorID: "author", CommentsEnabled: true,
		CreatedAt: time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)}
}
