package graphql_test

import (
	"context"
	"fmt"
	"testing"

	"OzonBankTestTask/internal/dto"
	"OzonBankTestTask/internal/graphql/limits"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGraphQLRejectsExplicitNullAndUnknownInput(t *testing.T) {
	queries := []string{
		`{ posts(first:null) { nodes { id } } }`,
		`mutation { createPost(input:{title:"title", content:"body", commentsEnabled:null}) { id } }`,
		`mutation { createPost(input:{title:"title", content:"body", authorId:"spoof"}) { id } }`,
		`mutation { createComment(input:{postId:"id"}) { id } }`,
	}
	for _, query := range queries {
		h, _, _ := newServer(t, limits.Config{})
		_, response := request(t, h, query, nil, "author")
		assertCode(t, response, "GRAPHQL_VALIDATION_FAILED")
	}
	h, _, _ := newServer(t, limits.Config{})
	_, response := request(t, h, `mutation($input:CreatePostInput!){createPost(input:$input){id}}`, map[string]any{
		"input": map[string]any{"title": "title", "content": "body", "commentsEnabled": nil},
	}, "author")
	assertCode(t, response, "GRAPHQL_VALIDATION_FAILED")
}

func TestPanicsAreHiddenIncludingBatchFetch(t *testing.T) {
	for _, batch := range []bool{false, true} {
		h, posts, _ := newServer(t, limits.Config{})
		query := `{ posts { nodes { id } } }`
		if batch {
			query = fmt.Sprintf(`{ post(id:%q) { id } }`, uuid.NewString())
			posts.On("GetMany", mock.Anything, mock.Anything).Panic("secret panic").Once()
		} else {
			posts.On("List", mock.Anything, mock.Anything).Panic("secret panic").Once()
		}
		w, response := request(t, h, query, nil, "")
		assertCode(t, response, "INTERNAL")
		require.NotContains(t, w.Body.String(), "secret")
	}
}

func TestIDAliasesBatchAndCacheIsIsolatedPerRequest(t *testing.T) {
	h, posts, _ := newServer(t, limits.Config{})
	post, missingID := examplePost(), uuid.NewString()
	posts.On("GetMany", mock.Anything, mock.Anything).Return(func(ctx context.Context, ids []string) ([]*dto.Post, error) {
		require.ElementsMatch(t, []string{post.ID, missingID}, ids)
		result := make([]*dto.Post, len(ids))
		for i, id := range ids {
			if id == post.ID {
				copyPost := post
				result[i] = &copyPost
			}
		}
		return result, nil
	}).Twice()
	query := fmt.Sprintf(`{ a:post(id:%q){id} b:post(id:%q){id} missing:post(id:%q){id} bad:post(id:"bad"){id} }`, post.ID, post.ID, missingID)
	for i := 0; i < 2; i++ {
		_, response := request(t, h, query, nil, "")
		assertCode(t, response, "BAD_USER_INPUT")
		require.JSONEq(t, string(response.Data["a"]), string(response.Data["b"]))
		require.JSONEq(t, "null", string(response.Data["missing"]))
		require.Contains(t, string(response.Data["a"]), post.ID)
	}
}

func TestPageSizeValidatedBeforeExecution(t *testing.T) {
	for _, size := range []int{-1, 0, 101} {
		h, _, _ := newServer(t, limits.Config{})
		_, response := request(t, h, `query($first:Int!){posts(first:$first){nodes{id}}}`, map[string]any{"first": size}, "")
		assertCode(t, response, "BAD_USER_INPUT")
	}
	h, _, _ := newServer(t, limits.Config{})
	_, response := request(t, h, fmt.Sprintf(`{post(id:%q){comments(first:101){nodes{id}}}}`, uuid.NewString()), nil, "")
	assertCode(t, response, "BAD_USER_INPUT")
}
