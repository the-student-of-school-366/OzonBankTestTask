package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

	"OzonBankTestTask/internal/infrastructure/middleware"
	"OzonBankTestTask/internal/requestctx"

	"github.com/stretchr/testify/require"
)

func TestTimeoutPassesSuccessfulResponse(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		contexts := make(chan context.Context, 1)
		h := middleware.Timeout(time.Second, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			contexts <- r.Context()
			w.Header().Set("X-Handler", "called")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("created"))
		}))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", nil))
		require.Equal(t, http.StatusCreated, w.Code)
		require.Equal(t, "called", w.Header().Get("X-Handler"))
		require.Equal(t, "created", w.Body.String())
		require.ErrorIs(t, (<-contexts).Err(), context.Canceled)
	})
}

func TestTimeoutCancelsContextWithoutWaitingForHandler(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		defer close(release)
		contexts := make(chan context.Context, 1)
		h := middleware.Timeout(time.Second, func(r *http.Request) string {
			return `{"requestId":"` + requestctx.RequestID(r.Context()) + `"}`
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			contexts <- r.Context()
			<-release
		}))
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r = r.WithContext(requestctx.WithRequestID(r.Context(), "test-request-id"))
		w := httptest.NewRecorder()
		w.Header().Set("Content-Type", "application/json")
		h.ServeHTTP(w, r)
		require.Equal(t, http.StatusServiceUnavailable, w.Code)
		require.JSONEq(t, `{"requestId":"test-request-id"}`, w.Body.String())
		require.Equal(t, "application/json", w.Header().Get("Content-Type"))
		require.ErrorIs(t, (<-contexts).Err(), context.DeadlineExceeded)
	})
}
