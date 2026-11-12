package graphql_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"OzonBankTestTask/internal/dto"
	gqlapi "OzonBankTestTask/internal/graphql"
	"OzonBankTestTask/internal/graphql/limits"
	"OzonBankTestTask/internal/graphql/mocks"
	"OzonBankTestTask/internal/requestctx"

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

func newServer(t *testing.T, config limits.Config) (http.Handler, *mocks.PostUseCase, *mocks.CommentUseCase) {
	t.Helper()
	posts, comments := mocks.NewPostUseCase(t), mocks.NewCommentUseCase(t)
	if config.MaxDepth == 0 {
		config.MaxDepth = 20
	}
	if config.MaxComplexity == 0 {
		config.MaxComplexity = 10_000
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := gqlapi.NewServer(posts, comments, config, logger)
	return h, posts, comments
}

func request(t *testing.T, h http.Handler, query string, variables map[string]any, actor string) (*httptest.ResponseRecorder, graphResponse) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	require.NoError(t, err)
	r := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	ctx := requestctx.WithRequestID(r.Context(), "test-request-id")
	if actor != "" {
		ctx = requestctx.WithActor(ctx, actor)
	}
	r = r.WithContext(ctx)
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

func exampleComment(postID string) dto.Comment {
	return dto.Comment{ID: uuid.NewString(), PostID: postID, AuthorID: "reader", Text: "text",
		CreatedAt: time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)}
}
