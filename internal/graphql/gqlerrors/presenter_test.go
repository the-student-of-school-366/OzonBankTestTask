package gqlerrors_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/graphql/gqlerrors"
	"OzonBankTestTask/internal/requestctx"

	"github.com/stretchr/testify/require"
)

func TestGraphQLErrorMapping(t *testing.T) {
	present := gqlerrors.NewPresenter(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := requestctx.WithRequestID(context.Background(), "test-request-id")
	for _, tc := range []struct {
		err  error
		code string
	}{
		{domain.ErrNotAuthor, "FORBIDDEN"}, {domain.ErrPostNotFound, "NOT_FOUND"},
		{domain.ErrCommentNotFound, "NOT_FOUND"}, {domain.ErrCommentsDisabled, "COMMENTS_DISABLED"},
		{domain.ErrParentPostMismatch, "PARENT_POST_MISMATCH"}, {domain.ErrInvalidCursor, "BAD_USER_INPUT"},
		{domain.ErrCommentTooLong, "BAD_USER_INPUT"}, {domain.ErrStorage, "INTERNAL"},
		{errors.New("driver password=secret"), "INTERNAL"}, {domain.ErrInvalidTime, "INTERNAL"},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			result := present(ctx, fmt.Errorf("private driver context: %w", tc.err))
			require.Equal(t, tc.code, result.Extensions["code"])
			require.Equal(t, "test-request-id", result.Extensions["requestId"])
			body, err := json.Marshal(result)
			require.NoError(t, err)
			require.NotContains(t, string(body), "private")
			require.NotContains(t, string(body), "secret")
		})
	}
}
