package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"OzonBankTestTask/internal/api"
	"OzonBankTestTask/internal/api/handler"
	domaincomment "OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/domain/page"
	domainpost "OzonBankTestTask/internal/domain/post"
	"OzonBankTestTask/internal/dto"
	"OzonBankTestTask/internal/infrastructure/pubsub"
	"OzonBankTestTask/internal/usecase"
	commentusecase "OzonBankTestTask/internal/usecase/comment"
	commentmocks "OzonBankTestTask/internal/usecase/comment/mocks"
	postusecase "OzonBankTestTask/internal/usecase/post"
	postmocks "OzonBankTestTask/internal/usecase/post/mocks"
	"OzonBankTestTask/internal/usecase/subscription"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHTTPWithRealUseCases(t *testing.T) {
	postRepository, commentRepository := postmocks.NewRepository(t), commentmocks.NewRepository(t)
	now := time.Date(2026, 11, 12, 15, 0, 0, 123456000, time.FixedZone("MSK", 3*3600))
	posts := postusecase.New(postRepository, usecase.WithClock(func() time.Time { return now }))
	hub, err := pubsub.New(pubsub.Config{})
	require.NoError(t, err)
	t.Cleanup(hub.Close)
	comments := commentusecase.New(commentRepository, hub, usecase.WithClock(func() time.Time { return now }))
	subscriptions := subscription.New(posts, hub)
	h, err := handler.NewHandler(posts, comments, subscriptions, api.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	require.NoError(t, err)
	t.Cleanup(h.Close)
	var saved domainpost.Post
	postRepository.On("CreatePost", mock.Anything, mock.Anything).Return(func(ctx context.Context, value domainpost.Post) (domainpost.Post, error) {
		saved = value
		return value, nil
	}).Once()
	_, created := request(t, h, `mutation{createPost(input:{title:"post",content:"body"}){id authorId commentsEnabled createdAt}}`, nil, "author")
	require.Empty(t, created.Errors)
	var post dto.Post
	require.NoError(t, json.Unmarshal(created.Data["createPost"], &post))
	require.Equal(t, saved.ID.String(), post.ID)
	require.True(t, post.CommentsEnabled)
	require.Equal(t, now.UTC(), post.CreatedAt)

	commentRepository.On("CreateComment", mock.Anything, mock.Anything).Return(func(ctx context.Context, value domaincomment.Comment) (domaincomment.Comment, error) {
		return value, nil
	}).Twice()
	_, rootResponse := request(t, h, fmt.Sprintf(`mutation{createComment(input:{postId:%q,text:"root"}){id parentId postId text authorId}}`, post.ID), nil, "reader")
	require.Empty(t, rootResponse.Errors)
	var root dto.Comment
	require.NoError(t, json.Unmarshal(rootResponse.Data["createComment"], &root))
	require.Nil(t, root.ParentID)
	require.Equal(t, "reader", root.AuthorID)
	_, replyResponse := request(t, h, fmt.Sprintf(`mutation{createComment(input:{postId:%q,parentId:%q,text:"reply"}){id parentId}}`, post.ID, root.ID), nil, "reader")
	require.Empty(t, replyResponse.Errors)
	var reply dto.Comment
	require.NoError(t, json.Unmarshal(replyResponse.Data["createComment"], &reply))
	require.Equal(t, root.ID, *reply.ParentID)
	_, invalid := request(t, h, `mutation($input:CreateCommentInput!){createComment(input:$input){id}}`, map[string]any{
		"input": map[string]any{"postId": post.ID, "text": strings.Repeat("я", 2001)},
	}, "reader")
	assertCode(t, invalid, "BAD_USER_INPUT")

	position := saved.PagePosition()
	postRepository.On("ListPosts", mock.Anything, mock.MatchedBy(func(p page.Request) bool { return p.First == 1 && p.After == nil })).
		Return(page.Page[domainpost.Post]{Items: []domainpost.Post{saved}, End: &position, HasNextPage: true}, nil).Once()
	_, first := request(t, h, `{posts(first:1){nodes{id} pageInfo{endCursor hasNextPage}}}`, nil, "")
	require.Empty(t, first.Errors)
	var connection dto.PostConnection
	require.NoError(t, json.Unmarshal(first.Data["posts"], &connection))
	require.True(t, connection.PageInfo.HasNextPage)
	require.NotNil(t, connection.PageInfo.EndCursor)
	postRepository.On("ListPosts", mock.Anything, mock.MatchedBy(func(p page.Request) bool {
		return p.First == 1 && p.After != nil && p.After.ID == saved.ID && p.After.CreatedAt.Equal(saved.CreatedAt)
	})).
		Return(page.Page[domainpost.Post]{Items: []domainpost.Post{}}, nil).Once()
	_, last := request(t, h, `query($cursor:String!){posts(first:1,after:$cursor){nodes{id} pageInfo{endCursor hasNextPage}}}`, map[string]any{"cursor": *connection.PageInfo.EndCursor}, "")
	require.Empty(t, last.Errors)
	require.JSONEq(t, `{"nodes":[],"pageInfo":{"endCursor":null,"hasNextPage":false}}`, string(last.Data["posts"]))
}
