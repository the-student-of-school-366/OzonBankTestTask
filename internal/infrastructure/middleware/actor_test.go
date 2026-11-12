package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"OzonBankTestTask/internal/infrastructure/middleware"
	"OzonBankTestTask/internal/requestctx"

	"github.com/stretchr/testify/require"
)

func TestDemoActorPreservesTrustedIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, header, trusted, want string
		hasTrusted                  bool
	}{
		{name: "anonymous"},
		{name: "demo header", header: "reader", want: "reader"},
		{name: "trusted overrides header", header: "spoofed", trusted: "author", hasTrusted: true, want: "author"},
		{name: "trusted without header", trusted: "author", hasTrusted: true, want: "author"},
		{name: "empty trusted actor blocks header", header: "spoofed", hasTrusted: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			h := middleware.DemoActor(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				actor, err := requestctx.Actor(r.Context())
				if tc.want == "" {
					require.ErrorIs(t, err, requestctx.ErrUnauthenticated)
				} else {
					require.NoError(t, err)
				}
				require.Equal(t, tc.want, actor)
				w.WriteHeader(http.StatusNoContent)
			}))
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set("X-User-ID", tc.header)
			if tc.hasTrusted {
				r = r.WithContext(requestctx.WithActor(r.Context(), tc.trusted))
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			require.True(t, called)
			require.Equal(t, http.StatusNoContent, w.Code)
			require.Equal(t, tc.hasTrusted, requestctx.HasActor(r.Context()))
		})
	}
}
