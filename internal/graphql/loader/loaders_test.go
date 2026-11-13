package loader

import (
	"context"
	"sync"
	"testing"
	"time"

	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/dto"
	"OzonBankTestTask/internal/graphql/loader/mocks"
	"OzonBankTestTask/internal/pagination"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPageCacheIncludesFirstAndCursor(t *testing.T) {
	posts, comments := mocks.NewPostReader(t), mocks.NewCommentReader(t)
	loaders := New(posts, comments)
	postID := uuid.New()
	cursor, err := pagination.Encode(page.Position{ID: uuid.New(), CreatedAt: time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)}, pagination.Scope{Kind: pagination.Roots, ID: postID})
	require.NoError(t, err)
	var seen []dto.RootPageInput
	comments.On("ListRootsBatch", mock.Anything, mock.Anything).Return(func(ctx context.Context, inputs []dto.RootPageInput) ([]dto.CommentConnection, error) {
		seen = append(seen, inputs...)
		result := make([]dto.CommentConnection, len(inputs))
		for i, input := range inputs {
			for j := 0; j < *input.First; j++ {
				comment := dto.Comment{ID: uuid.NewString(), PostID: input.PostID, Text: "first page"}
				if input.After != nil {
					comment.Text = "next page"
				}
				result[i].Nodes = append(result[i].Nodes, comment)
			}
		}
		return result, nil
	})
	ctx := context.Background()
	a, err := loaders.Roots(ctx, postID.String(), 1, nil)
	require.NoError(t, err)
	b, err := loaders.Roots(ctx, postID.String(), 2, nil)
	require.NoError(t, err)
	c, err := loaders.Roots(ctx, postID.String(), 1, nil)
	require.NoError(t, err)
	d, err := loaders.Roots(ctx, postID.String(), 1, &cursor)
	require.NoError(t, err)
	require.Equal(t, a, c)
	require.Len(t, b.Nodes, 2)
	require.Equal(t, "next page", d.Nodes[0].Text)
	require.Len(t, seen, 3)
}

func TestLoaderSplitsLargeBatch(t *testing.T) {
	posts, comments := mocks.NewPostReader(t), mocks.NewCommentReader(t)
	loaders := New(posts, comments)
	ids := make([]string, 201)
	for i := range ids {
		ids[i] = uuid.NewString()
	}
	var mu sync.Mutex
	var batches [][]string
	posts.On("GetMany", mock.Anything, mock.Anything).Return(func(ctx context.Context, ids []string) ([]*dto.Post, error) {
		mu.Lock()
		batches = append(batches, append([]string(nil), ids...))
		mu.Unlock()
		return make([]*dto.Post, len(ids)), nil
	})
	values, err := loaders.posts.LoadAll(context.Background(), ids)
	require.NoError(t, err)
	require.Len(t, values, len(ids))
	mu.Lock()
	defer mu.Unlock()
	var seen []string
	for _, batch := range batches {
		require.LessOrEqual(t, len(batch), page.MaxBatchSize)
		seen = append(seen, batch...)
	}
	require.ElementsMatch(t, ids, seen)
}
