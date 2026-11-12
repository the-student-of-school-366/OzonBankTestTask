package dto

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/domain/post"
	"OzonBankTestTask/internal/pagination"

	"github.com/google/uuid"
)

func TestPageInput(t *testing.T) {
	scope := pagination.Scope{Kind: pagination.Roots, ID: uuid.New()}
	request, err := (PageInput{}).ToRequest(scope)
	if err != nil || request.First != 20 || request.After != nil {
		t.Fatalf("значения по умолчанию: %+v %v", request, err)
	}
	position := page.Position{CreatedAt: time.Unix(100, 0), ID: uuid.New()}
	cursor, err := pagination.Encode(position, scope)
	if err != nil {
		t.Fatal(err)
	}
	first := 100
	request, err = (PageInput{First: &first, After: &cursor}).ToRequest(scope)
	if err != nil || request.First != 100 || request.After == nil || request.After.Compare(position) != 0 {
		t.Fatalf("параметры: %+v %v", request, err)
	}
	for _, size := range []int{-1, 0, 101} {
		if _, err := (PageInput{First: &size}).ToRequest(scope); !errors.Is(err, domain.ErrInvalidPageSize) {
			t.Fatalf("размер %d: %v", size, err)
		}
	}
	empty := ""
	if _, err := (PageInput{After: &empty}).ToRequest(scope); !errors.Is(err, domain.ErrInvalidCursor) {
		t.Fatal(err)
	}
	if _, err := (PageInput{After: &cursor}).ToRequest(pagination.Scope{Kind: pagination.Roots, ID: uuid.New()}); !errors.Is(err, domain.ErrInvalidCursor) {
		t.Fatal(err)
	}
	if _, err := (PageInput{}).ToRequest(pagination.Scope{}); !errors.Is(err, domain.ErrInvalidCursor) {
		t.Fatal(err)
	}
}

func TestConnectionsAndNextRequest(t *testing.T) {
	var posts []post.Post
	for i := 0; i < 3; i++ {
		postItem, err := (CreatePostInput{Title: "title", Content: "content"}).ToDomain("actor", time.Unix(int64(100-i), 0))
		if err != nil {
			t.Fatal(err)
		}
		posts = append(posts, *postItem)
	}
	postPage, err := page.CutPage(posts, 2)
	if err != nil {
		t.Fatal(err)
	}
	connection, err := PostConnectionFromDomain(postPage)
	if err != nil {
		t.Fatal(err)
	}
	if len(connection.Nodes) != 2 || !connection.PageInfo.HasNextPage || connection.PageInfo.EndCursor == nil {
		t.Fatalf("страница: %+v", connection)
	}
	if connection.Nodes[0].AuthorID != "actor" || connection.Nodes[0].ID != posts[0].ID.String() {
		t.Fatalf("пост: %+v", connection.Nodes[0])
	}
	request, err := (PageInput{After: connection.PageInfo.EndCursor}).ToRequest(pagination.Scope{Kind: pagination.Posts})
	if err != nil {
		t.Fatal(err)
	}
	if request.After.ID != posts[1].ID || posts[2].PagePosition().Compare(*request.After) >= 0 {
		t.Fatal("курсором должен быть последний выданный пост")
	}
	connection.Nodes[0].Title = "changed"
	if posts[0].Title == "changed" {
		t.Fatal("DTO изменяет доменную модель")
	}

	parent, err := comment.NewComment(posts[0].ID, nil, "reader", "root", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	reply, err := comment.NewComment(posts[0].ID, &parent.ID, "reader", "reply", time.Unix(2, 0))
	if err != nil {
		t.Fatal(err)
	}
	commentPage, err := page.CutPage([]comment.Comment{*reply}, 20)
	if err != nil {
		t.Fatal(err)
	}
	scope := pagination.Scope{Kind: pagination.Replies, ID: parent.ID}
	comments, err := CommentConnectionFromDomain(commentPage, scope)
	if err != nil {
		t.Fatal(err)
	}
	if comments.PageInfo.HasNextPage || len(comments.Nodes) != 1 || comments.Nodes[0].ParentID == nil || *comments.Nodes[0].ParentID != parent.ID.String() {
		t.Fatalf("ответы: %+v", comments)
	}
	position, err := pagination.Decode(*comments.PageInfo.EndCursor, scope)
	if err != nil || position.ID != reply.ID {
		t.Fatalf("позиция ответа: %v", err)
	}
	*comments.Nodes[0].ParentID = "changed"
	if *reply.ParentID != parent.ID {
		t.Fatal("DTO изменяет ParentID домена")
	}
	if _, err := CommentConnectionFromDomain(commentPage, pagination.Scope{Kind: pagination.Posts}); !errors.Is(err, domain.ErrInvalidCursor) {
		t.Fatal(err)
	}
	rootDTO := CommentFromDomain(*parent)
	if rootDTO.ParentID != nil {
		t.Fatal("у корня появился parentId")
	}
}

func TestEmptyConnectionsJSON(t *testing.T) {
	posts, err := PostConnectionFromDomain(page.Page[post.Post]{})
	if err != nil {
		t.Fatal(err)
	}
	comments, err := CommentConnectionFromDomain(page.Page[comment.Comment]{}, pagination.Scope{Kind: pagination.Roots, ID: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	for _, connection := range []any{posts, comments} {
		data, err := json.Marshal(connection)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != `{"nodes":[],"pageInfo":{"endCursor":null,"hasNextPage":false}}` {
			t.Fatalf("пустая страница: %s", data)
		}
	}
}
