package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"OzonBankTestTask/internal/api"
	"OzonBankTestTask/internal/dto"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTrustedActorOverridesDemoHeader(t *testing.T) {
	h, posts, _ := newHandler(t, api.Config{})
	posts.On("Create", mock.Anything, "trusted", mock.Anything).Return(examplePost(), nil).Once()
	trusted := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(api.WithActor(r.Context(), "trusted")))
	})
	_, response := request(t, trusted, `mutation { createPost(input:{title:"title", content:"body"}) { id } }`, nil, "spoofed")
	require.Empty(t, response.Errors)
}

func TestHTTPTransportBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, method, contentType, body string
		status                          int
		code                            string
	}{
		{"method", "DELETE", "application/json", `{}`, 405, "BAD_USER_INPUT"},
		{"content type", "POST", "text/plain", `{}`, 415, "BAD_USER_INPUT"},
		{"invalid JSON", "POST", "application/json", `{`, 400, "BAD_USER_INPUT"},
		{"batch unsupported", "POST", "application/json", `[]`, 400, "BAD_USER_INPUT"},
		{"trailing JSON", "POST", "application/json", `{} {}`, 400, "BAD_USER_INPUT"},
		{"size", "POST", "application/json", strings.Repeat("x", 257), 413, "PAYLOAD_TOO_LARGE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, _ := newHandler(t, api.Config{MaxBodyBytes: 256})
			r := httptest.NewRequest(tc.method, "/graphql", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", tc.contentType)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			require.Equal(t, tc.status, w.Code)
			var response graphResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
			assertCode(t, response, tc.code)
			require.NotEmpty(t, w.Header().Get("X-Request-ID"))
			require.Equal(t, w.Header().Get("X-Request-ID"), response.Errors[0].Extensions["requestId"])
		})
	}
}

func TestTimeoutCancelsUseCase(t *testing.T) {
	h, posts, _ := newHandler(t, api.Config{RequestTimeout: 30 * time.Millisecond})
	finished := make(chan struct{})
	posts.On("List", mock.Anything, mock.Anything).Return(func(ctx context.Context, input dto.PageInput) (dto.PostConnection, error) {
		<-ctx.Done()
		close(finished)
		return dto.PostConnection{}, ctx.Err()
	}).Once()
	w, response := request(t, h, `{ posts { nodes { id } } }`, nil, "")
	assertCode(t, response, "TIMEOUT")
	require.NotEmpty(t, w.Header().Get("X-Request-ID"))
	require.Equal(t, w.Header().Get("X-Request-ID"), response.Errors[0].Extensions["requestId"])
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("usecase context was not canceled")
	}
}

func TestQueryDefaultsAndGETTransport(t *testing.T) {
	h, posts, _ := newHandler(t, api.Config{})
	posts.On("List", mock.Anything, mock.MatchedBy(func(input dto.PageInput) bool { return input.First != nil && *input.First == 20 })).
		Return(dto.PostConnection{Nodes: []dto.Post{}}, nil).Once()
	query := url.Values{"query": {`{posts{nodes{id} pageInfo{endCursor hasNextPage}}}`}}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/graphql?"+query.Encode(), nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"data":{"posts":{"nodes":[],"pageInfo":{"endCursor":null,"hasNextPage":false}}}}`, w.Body.String())
	query.Set("query", `mutation{createPost(input:{title:"a",content:"b"}){id}}`)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/graphql?"+query.Encode(), nil))
	require.Equal(t, http.StatusNotAcceptable, w.Code)
}

func TestHTTPTimeoutDoesNotWaitForNonCooperativeDependency(t *testing.T) {
	h, posts, _ := newHandler(t, api.Config{RequestTimeout: 50 * time.Millisecond})
	release, completed := make(chan struct{}), make(chan struct{})
	defer func() {
		close(release)
		select {
		case <-completed:
		case <-time.After(time.Second):
			t.Error("dependency did not finish")
		}
	}()
	posts.On("List", mock.Anything, mock.Anything).Return(func(ctx context.Context, input dto.PageInput) (dto.PostConnection, error) {
		defer close(completed)
		<-release
		return dto.PostConnection{}, ctx.Err()
	}).Once()
	w, response := request(t, h, `{posts{nodes{id}}}`, nil, "")
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	assertCode(t, response, "TIMEOUT")
	require.Equal(t, w.Header().Get("X-Request-ID"), response.Errors[0].Extensions["requestId"])
	require.Contains(t, w.Header().Get("Content-Type"), "application/json")
}
