package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"OzonBankTestTask/internal/infrastructure/middleware"
	"OzonBankTestTask/internal/requestctx"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRequestIDMatchesContextAndResponse(t *testing.T) {
	var ids []string
	h := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids = append(ids, requestctx.RequestID(r.Context()))
		w.WriteHeader(http.StatusNoContent)
	}))
	for i := 0; i < 2; i++ {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-Request-ID", "client-supplied-id")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		require.Equal(t, http.StatusNoContent, w.Code)
		require.Len(t, ids, i+1)
		_, err := uuid.Parse(ids[i])
		require.NoError(t, err)
		require.Equal(t, ids[i], w.Header().Get("X-Request-ID"))
		require.NotEqual(t, "client-supplied-id", ids[i])
		require.Empty(t, requestctx.RequestID(r.Context()))
	}
	require.NotEqual(t, ids[0], ids[1])
}
