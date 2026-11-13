package sandbox_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"OzonBankTestTask/internal/api/sandbox"

	"github.com/stretchr/testify/require"
)

func TestSandboxMethods(t *testing.T) {
	h := sandbox.NewHandler()
	for _, tc := range []struct {
		method string
		status int
	}{{http.MethodGet, http.StatusOK}, {http.MethodHead, http.StatusOK}, {http.MethodPost, http.StatusMethodNotAllowed}} {
		t.Run(tc.method, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(tc.method, "/sandbox", nil))
			require.Equal(t, tc.status, w.Code)
			if tc.method == http.MethodGet {
				require.Contains(t, w.Body.String(), "EmbeddedSandbox")
			}
			if tc.method == http.MethodHead {
				require.Empty(t, w.Body.String())
			}
			if tc.method == http.MethodPost {
				require.Equal(t, "GET, HEAD", w.Header().Get("Allow"))
			}
		})
	}
}
