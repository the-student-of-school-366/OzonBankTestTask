package graphql_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"OzonBankTestTask/internal/dto"
	"OzonBankTestTask/internal/graphql/limits"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCreatePostDefaultsAndActor(t *testing.T) {
	for _, tc := range []struct {
		name    string
		enabled any
		include bool
		want    bool
	}{
		{"default", nil, false, true}, {"false", false, true, false}, {"true", true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, posts, _ := newServer(t, limits.Config{})
			posts.On("Create", mock.Anything, "author", mock.Anything).Return(func(ctx context.Context, actor string, input dto.CreatePostInput) (dto.Post, error) {
				require.NotNil(t, input.CommentsEnabled)
				require.Equal(t, tc.want, *input.CommentsEnabled)
				require.Equal(t, "title", input.Title)
				value := examplePost()
				value.CommentsEnabled = *input.CommentsEnabled
				return value, nil
			}).Once()
			input := map[string]any{"title": "title", "content": "body"}
			if tc.include {
				input["commentsEnabled"] = tc.enabled
			}
			w, response := request(t, h, `mutation($input: CreatePostInput!) { createPost(input: $input) { id authorId commentsEnabled createdAt } }`, map[string]any{"input": input}, "author")
			require.Empty(t, response.Errors)
			require.Equal(t, http.StatusOK, w.Code)
			var post dto.Post
			require.NoError(t, json.Unmarshal(response.Data["createPost"], &post))
			require.Equal(t, tc.want, post.CommentsEnabled)
			require.Equal(t, "author", post.AuthorID)
			require.False(t, post.CreatedAt.IsZero())
		})
	}
}

func TestMutationRequiresActor(t *testing.T) {
	for _, query := range []string{
		`mutation { createPost(input: {title:"title", content:"body"}) { id } }`,
		fmt.Sprintf(`mutation { createComment(input: {postId:%q, text:"text"}) { id } }`, uuid.NewString()),
		fmt.Sprintf(`mutation { setCommentsEnabled(postId:%q, enabled:false) { id } }`, uuid.NewString()),
	} {
		h, _, _ := newServer(t, limits.Config{})
		_, response := request(t, h, query, nil, "")
		assertCode(t, response, "UNAUTHENTICATED")
	}
}

func TestQueryMissingAndInvalidIDs(t *testing.T) {
	h, posts, comments := newServer(t, limits.Config{})
	postID, commentID := uuid.NewString(), uuid.NewString()
	posts.On("GetMany", mock.Anything, []string{postID}).Return([]*dto.Post{nil}, nil).Once()
	comments.On("GetMany", mock.Anything, []string{commentID}).Return([]*dto.Comment{nil}, nil).Once()
	_, response := request(t, h, fmt.Sprintf(`{ post(id:%q){id} comment(id:%q){id} }`, postID, commentID), nil, "")
	require.Empty(t, response.Errors)
	require.JSONEq(t, "null", string(response.Data["post"]))
	require.JSONEq(t, "null", string(response.Data["comment"]))
	_, response = request(t, h, `{ post(id:"invalid"){id} }`, nil, "")
	assertCode(t, response, "BAD_USER_INPUT")
	require.Equal(t, []any{"post"}, response.Errors[0].Path)
}

func TestNestedListsAreBatched(t *testing.T) {
	for _, size := range []int{1, 10, 50} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			h, posts, comments := newServer(t, limits.Config{})
			values := make([]dto.Post, size)
			for i := range values {
				values[i] = examplePost()
			}
			posts.On("List", mock.Anything, mock.Anything).Return(dto.PostConnection{Nodes: values}, nil).Once()
			var mu sync.Mutex
			var rootCalls, replyCalls, rootKeys, replyKeys int
			comments.On("ListRootsBatch", mock.Anything, mock.Anything).Return(func(ctx context.Context, inputs []dto.RootPageInput) ([]dto.CommentConnection, error) {
				mu.Lock()
				defer mu.Unlock()
				rootCalls++
				rootKeys += len(inputs)
				result := make([]dto.CommentConnection, len(inputs))
				for i, input := range inputs {
					require.Equal(t, 1, *input.First)
					result[i].Nodes = []dto.Comment{exampleComment(input.PostID)}
				}
				return result, nil
			})
			comments.On("ListRepliesBatch", mock.Anything, mock.Anything).Return(func(ctx context.Context, inputs []dto.ReplyPageInput) ([]dto.CommentConnection, error) {
				mu.Lock()
				defer mu.Unlock()
				replyCalls++
				replyKeys += len(inputs)
				return make([]dto.CommentConnection, len(inputs)), nil
			})
			_, response := request(t, h, fmt.Sprintf(`{ posts(first:%d) { nodes { id comments(first:1) { nodes { id replies(first:1) { nodes {id} } } } } } }`, size), nil, "")
			require.Empty(t, response.Errors)
			mu.Lock()
			defer mu.Unlock()
			require.Equal(t, size, rootKeys)
			require.Equal(t, size, replyKeys)
			// Допускается разделение по временному окну, но не по одному вызову на родителя.
			require.LessOrEqual(t, rootCalls, 2)
			require.LessOrEqual(t, replyCalls, 2)
		})
	}
}

func TestMutationInvalidatesPageCache(t *testing.T) {
	h, posts, comments := newServer(t, limits.Config{})
	post := examplePost()
	var mu sync.Mutex
	state := true
	posts.On("SetCommentsEnabled", mock.Anything, "author", mock.Anything).Return(func(ctx context.Context, actor string, input dto.SetCommentsEnabledInput) (dto.Post, error) {
		mu.Lock()
		defer mu.Unlock()
		state = input.Enabled
		value := post
		value.CommentsEnabled = state
		return value, nil
	}).Twice()
	comments.On("ListRootsBatch", mock.Anything, mock.Anything).Return(func(ctx context.Context, inputs []dto.RootPageInput) ([]dto.CommentConnection, error) {
		mu.Lock()
		defer mu.Unlock()
		comment := exampleComment(post.ID)
		comment.Text = fmt.Sprint(state)
		return []dto.CommentConnection{{Nodes: []dto.Comment{comment}}}, nil
	}).Twice()
	_, response := request(t, h, fmt.Sprintf(`mutation {
		before:setCommentsEnabled(postId:%q,enabled:true){comments(first:1){nodes{text}}}
		after:setCommentsEnabled(postId:%q,enabled:false){comments(first:1){nodes{text}}}
	}`, post.ID, post.ID), nil, "author")
	require.Empty(t, response.Errors)
	require.Contains(t, string(response.Data["before"]), `"true"`)
	require.Contains(t, string(response.Data["after"]), `"false"`)
}
