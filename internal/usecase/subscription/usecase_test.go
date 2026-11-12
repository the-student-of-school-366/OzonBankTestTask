package subscription_test

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/dto"
	"OzonBankTestTask/internal/usecase/subscription"
	"OzonBankTestTask/internal/usecase/subscription/mocks"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestValidationBeforeSubscribing(t *testing.T) {
	posts, broker := mocks.NewPostReader(t), mocks.NewBroker(t)
	u := subscription.New(posts, broker)
	_, err := u.CommentAdded(context.Background(), "invalid")
	require.ErrorIs(t, err, domain.ErrInvalidID)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = u.CommentAdded(ctx, uuid.NewString())
	require.ErrorIs(t, err, context.Canceled)
}

func TestMissingPostAndReadErrorsDoNotSubscribe(t *testing.T) {
	for _, tc := range []struct {
		name      string
		posts     []*dto.Post
		err, want error
	}{
		{name: "missing", posts: []*dto.Post{nil}, want: domain.ErrPostNotFound},
		{name: "bad cardinality", want: domain.ErrStorage},
		{name: "read error", err: domain.ErrStorage, want: domain.ErrStorage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts, broker := mocks.NewPostReader(t), mocks.NewBroker(t)
			posts.On("GetMany", mock.Anything, mock.Anything).Return(tc.posts, tc.err).Once()
			_, err := subscription.New(posts, broker).CommentAdded(context.Background(), uuid.NewString())
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestBrokerErrors(t *testing.T) {
	for _, want := range []error{subscription.ErrClosed, subscription.ErrLimitReached} {
		posts, broker := mocks.NewPostReader(t), mocks.NewBroker(t)
		id := uuid.New()
		posts.On("GetMany", mock.Anything, []string{id.String()}).Return([]*dto.Post{{ID: id.String()}}, nil).Once()
		broker.On("Subscribe", mock.Anything, id).Return(subscription.Stream{}, want).Once()
		_, err := subscription.New(posts, broker).CommentAdded(context.Background(), id.String())
		require.ErrorIs(t, err, want)
	}
}

func TestEventsAndCancellationWhileConsumerIsBlocked(t *testing.T) {
	for _, stopBroker := range []bool{false, true} {
		synctest.Test(t, func(t *testing.T) {
			posts, broker := mocks.NewPostReader(t), mocks.NewBroker(t)
			postID, parentID := uuid.New(), uuid.New()
			posts.On("GetMany", mock.Anything, []string{postID.String()}).Return([]*dto.Post{{ID: postID.String(), CommentsEnabled: false}}, nil).Once()
			input, done, canceled := make(chan comment.Comment, 1), make(chan struct{}), make(chan struct{})
			broker.On("Subscribe", mock.Anything, postID).Return(subscription.Stream{Comments: input, Done: done, Cancel: func() { close(canceled) }}, nil).Once()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			output, err := subscription.New(posts, broker).CommentAdded(ctx, postID.String())
			require.NoError(t, err)
			for _, parent := range []*uuid.UUID{nil, &parentID} {
				value := comment.Comment{ID: uuid.New(), PostID: postID, ParentID: parent, Author: "author", Text: "text", CreatedAt: time.Now()}
				input <- value
				require.Equal(t, dto.CommentFromDomain(value), <-output)
			}
			input <- comment.Comment{ID: uuid.New(), PostID: postID}
			synctest.Wait()
			if stopBroker {
				close(done)
			} else {
				cancel()
			}
			synctest.Wait()
			<-canceled
			_, open := <-output
			require.False(t, open)
		})
	}
}
