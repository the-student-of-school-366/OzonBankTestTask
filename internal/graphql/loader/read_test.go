package loader_test

import (
	"context"
	"testing"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/graphql/loader"
	"OzonBankTestTask/internal/graphql/loader/mocks"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestInvalidPageCursorNeverEntersBatch(t *testing.T) {
	posts, comments := mocks.NewPostReader(t), mocks.NewCommentReader(t)
	loaders := loader.New(posts, comments)
	cursor := ""
	_, err := loaders.Roots(context.Background(), uuid.NewString(), 20, &cursor)
	require.ErrorIs(t, err, domain.ErrInvalidCursor)
	comments.AssertNotCalled(t, "ListRootsBatch")
}
