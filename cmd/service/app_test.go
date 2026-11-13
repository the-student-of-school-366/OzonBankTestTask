package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"OzonBankTestTask/internal/repositories/postgres/testdb"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

func graphqlRequest(t *testing.T, endpoint, query string) map[string]json.RawMessage {
	t.Helper()
	body, err := json.Marshal(map[string]string{"query": query})
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, endpoint, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "owner")
	response, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	var envelope struct {
		Data   map[string]json.RawMessage `json:"data"`
		Errors []json.RawMessage          `json:"errors"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&envelope))
	require.Empty(t, envelope.Errors)
	return envelope.Data
}

func createPost(t *testing.T, endpoint string) string {
	t.Helper()
	result := graphqlRequest(t, endpoint, `mutation { createPost(input:{title:"title",content:"content"}) { id } }`)
	var post struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(result["createPost"], &post))
	require.NotEmpty(t, post.ID)
	return post.ID
}

func TestMemoryApplicationAndGracefulShutdown(t *testing.T) {
	app, err := newApplication(t.Context(), config{storage: "memory"})
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		app.close()
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	finished := make(chan error, 1)
	go func() { finished <- app.serve(ctx, listener, 3*time.Second) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-finished:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Error("server did not shut down")
		}
	})
	endpoint := "http://" + listener.Addr().String() + "/graphql"
	id := createPost(t, endpoint)
	data := graphqlRequest(t, endpoint, fmt.Sprintf(`mutation { createComment(input:{postId:%q,text:"root"}) { text } }`, id))
	require.JSONEq(t, `{"text":"root"}`, string(data["createComment"]))
	data = graphqlRequest(t, endpoint, fmt.Sprintf(`{ post(id:%q) { comments(first:10) { nodes { text } } } }`, id))
	require.JSONEq(t, `{"comments":{"nodes":[{"text":"root"}]}}`, string(data["post"]))
	socketCtx, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	ws, _, err := websocket.Dial(socketCtx, "ws"+strings.TrimPrefix(endpoint, "http"), &websocket.DialOptions{Subprotocols: []string{"graphql-transport-ws"}})
	require.NoError(t, err)
	defer ws.CloseNow()
	cancel()
	_, _, err = ws.Read(socketCtx)
	require.Error(t, err)
	require.NoError(t, socketCtx.Err(), "WebSocket should close before the timeout")
}

func TestPostgresApplicationPersistsAcrossRestart(t *testing.T) {
	_, db := testdb.Open(t)
	configPath, err := filepath.Abs("../../internal/config/config.yaml")
	require.NoError(t, err)
	t.Setenv("CONFIG_FILE", configPath)
	t.Setenv("STORAGE", "")
	t.Setenv("DATABASE_URL", db.DSN)
	t.Setenv("SHUTDOWN_TIMEOUT", "")
	cfg, err := loadConfig()
	require.NoError(t, err)
	require.Equal(t, "postgres", cfg.storage)
	app, err := newApplication(t.Context(), cfg)
	require.NoError(t, err)
	defer app.close()
	server := httptest.NewServer(app.server.Handler)
	defer server.Close()
	id := createPost(t, server.URL+"/graphql")
	server.Close()
	app.close()
	app, err = newApplication(t.Context(), cfg)
	require.NoError(t, err)
	defer app.close()
	server = httptest.NewServer(app.server.Handler)
	defer server.Close()
	data := graphqlRequest(t, server.URL+"/graphql", fmt.Sprintf(`{post(id:%q){id title}}`, id))
	require.JSONEq(t, fmt.Sprintf(`{"id":%q,"title":"title"}`, id), string(data["post"]))
}

func TestInvalidStorageFailsStartup(t *testing.T) {
	_, err := newApplication(t.Context(), config{storage: "invalid"})
	require.Error(t, err)
}

func TestSandboxIsOptionalAndGraphQLRemainsAvailable(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("enabled=%t", enabled), func(t *testing.T) {
			app, err := newApplication(t.Context(), config{storage: "memory", sandboxEnabled: enabled})
			require.NoError(t, err)
			defer app.close()
			server := httptest.NewServer(app.server.Handler)
			defer server.Close()
			resp, err := http.Get(server.URL + "/sandbox")
			require.NoError(t, err)
			defer resp.Body.Close()
			if !enabled {
				require.Equal(t, http.StatusNotFound, resp.StatusCode)
			} else {
				require.Equal(t, http.StatusOK, resp.StatusCode)
				require.Contains(t, resp.Header.Get("Content-Type"), "text/html")
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.Contains(t, string(body), "embeddable-sandbox.cdn.apollographql.com")
				require.Contains(t, string(body), `window.location.origin + "/graphql"`)
			}
			id := createPost(t, server.URL+"/graphql")
			result := graphqlRequest(t, server.URL+"/graphql", fmt.Sprintf(`{post(id:%q){id}}`, id))
			require.JSONEq(t, fmt.Sprintf(`{"id":%q}`, id), string(result["post"]))
		})
	}
}
