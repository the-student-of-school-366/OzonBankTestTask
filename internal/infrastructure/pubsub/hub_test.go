package pubsub

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"

	"OzonBankTestTask/internal/broker"
	"OzonBankTestTask/internal/domain/comment"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPublishFiltersPostsAndCopiesEvents(t *testing.T) {
	hub, err := New(Config{})
	require.NoError(t, err)
	defer hub.Close()
	postID, parentID := uuid.New(), uuid.New()
	value := comment.Comment{ID: uuid.New(), PostID: postID, ParentID: &parentID}
	hub.Publish(value) // Нет replay для нового подписчика.
	a, err := hub.Subscribe(context.Background(), postID)
	require.NoError(t, err)
	b, err := hub.Subscribe(context.Background(), postID)
	require.NoError(t, err)
	other, err := hub.Subscribe(context.Background(), uuid.New())
	require.NoError(t, err)
	require.Empty(t, a.Comments)
	hub.Publish(value)
	first, second := <-a.Comments, <-b.Comments
	require.Equal(t, value, first)
	require.Equal(t, value, second)
	*first.ParentID = uuid.Nil
	require.Equal(t, parentID, *second.ParentID)
	require.NotEqual(t, uuid.Nil, parentID)
	require.Empty(t, other.Comments)
}

func TestSlowSubscriberDoesNotBlockOthers(t *testing.T) {
	hub, err := New(Config{BufferSize: 1})
	require.NoError(t, err)
	defer hub.Close()
	postID := uuid.New()
	slow, err := hub.Subscribe(context.Background(), postID)
	require.NoError(t, err)
	fast, err := hub.Subscribe(context.Background(), postID)
	require.NoError(t, err)
	hub.Publish(comment.Comment{PostID: postID, Text: "one"})
	require.Equal(t, "one", (<-fast.Comments).Text)
	hub.Publish(comment.Comment{PostID: postID, Text: "two"})
	select {
	case <-slow.Done:
	default:
		t.Fatal("slow subscriber was not closed")
	}
	require.Equal(t, "two", (<-fast.Comments).Text)
	require.Equal(t, 1, hub.count)
	slow.Cancel()
	slow.Cancel()
	require.Equal(t, 1, hub.count)
}

func TestLimitsCancellationAndShutdown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		hub, err := New(Config{MaxSubscriptions: 1})
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		stream, err := hub.Subscribe(ctx, uuid.New())
		require.NoError(t, err)
		_, err = hub.Subscribe(context.Background(), uuid.New())
		require.ErrorIs(t, err, broker.ErrLimitReached)
		cancel()
		synctest.Wait()
		<-stream.Done
		require.Zero(t, hub.count)
		_, err = hub.Subscribe(ctx, uuid.New())
		require.ErrorIs(t, err, context.Canceled)
		stream, err = hub.Subscribe(context.Background(), uuid.New())
		require.NoError(t, err)
		hub.Close()
		hub.Close()
		<-stream.Done
		_, open := <-stream.Comments
		require.False(t, open)
		_, err = hub.Subscribe(context.Background(), uuid.New())
		require.ErrorIs(t, err, broker.ErrClosed)
		hub.Publish(comment.Comment{PostID: uuid.New()})
		require.Zero(t, hub.count)
	})
}

func TestConcurrentPublishCancelAndClose(t *testing.T) {
	hub, err := New(Config{})
	require.NoError(t, err)
	postID := uuid.New()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		stream, err := hub.Subscribe(context.Background(), postID)
		require.NoError(t, err)
		wg.Add(2)
		go func() { defer wg.Done(); stream.Cancel(); stream.Cancel() }()
		go func() { defer wg.Done(); hub.Publish(comment.Comment{PostID: postID}) }()
	}
	hub.Close()
	wg.Wait()
	require.Zero(t, hub.count)
	require.Empty(t, hub.byPost)
}

func TestInvalidConfig(t *testing.T) {
	for _, config := range []Config{{BufferSize: -1}, {MaxSubscriptions: -1}} {
		_, err := New(config)
		require.Error(t, err)
	}
}
