package middleware_test

import (
	"bufio"
	"bytes"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"OzonBankTestTask/internal/infrastructure/middleware"
	"OzonBankTestTask/internal/requestctx"

	"github.com/stretchr/testify/require"
)

func TestAccessLogRecordsStatusWithoutRequestBody(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	h := middleware.RequestID(middleware.AccessLog(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NotEmpty(t, requestctx.RequestID(r.Context()))
		http.Error(w, "secret body", http.StatusTeapot)
	})))
	request := httptest.NewRequest(http.MethodPost, "/graphql?query=secret", nil)
	h.ServeHTTP(httptest.NewRecorder(), request)
	line := output.String()
	require.Contains(t, line, "http request")
	require.Contains(t, line, "status=418")
	require.Contains(t, line, "path=/graphql")
	require.NotContains(t, line, "secret")
}

func TestAccessLogKeepsHijack(t *testing.T) {
	base := &hijackWriter{ResponseWriter: httptest.NewRecorder()}
	h := middleware.AccessLog(slog.New(slog.NewTextHandler(io.Discard, nil)))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := http.NewResponseController(w).Hijack()
		require.NoError(t, err)
		require.Nil(t, conn)
	}))
	h.ServeHTTP(base, httptest.NewRequest(http.MethodGet, "/graphql", nil))
	require.True(t, base.hijacked)
}

type hijackWriter struct {
	http.ResponseWriter
	hijacked bool
}

func (w *hijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.hijacked = true
	return nil, nil, nil
}
