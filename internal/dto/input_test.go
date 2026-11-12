package dto

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"OzonBankTestTask/internal/domain"
	"github.com/google/uuid"
)

func TestCreatePostInput(t *testing.T) {
	for _, tt := range []struct {
		body    string
		enabled bool
	}{
		{`{"title":"title","content":"content"}`, true},
		{`{"title":"title","content":"content","commentsEnabled":false}`, false},
		{`{"title":"title","content":"content","commentsEnabled":true}`, true},
	} {
		var input CreatePostInput
		if err := json.Unmarshal([]byte(tt.body), &input); err != nil {
			t.Fatal(err)
		}
		post, err := input.ToDomain("actor-from-context", time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if post.CommentsEnabled != tt.enabled || post.Author != "actor-from-context" {
			t.Fatalf("неверный пост: %+v", post)
		}
	}
	if _, err := (CreatePostInput{Title: "title", Content: "content"}).ToDomain("", time.Now()); !errors.Is(err, domain.ErrEmptyAuthor) {
		t.Fatal(err)
	}
	if _, err := (CreatePostInput{}).ToDomain("author", time.Now()); !errors.Is(err, domain.ErrEmptyPostTitle) {
		t.Fatal(err)
	}
}

func TestCreateCommentInput(t *testing.T) {
	postID, parentID := uuid.New().String(), uuid.New().String()
	input := CreateCommentInput{PostID: postID, ParentID: &parentID, Text: " reply "}
	comment, err := input.ToDomain("actor", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if comment.PostID.String() != postID || comment.ParentID.String() != parentID || comment.Author != "actor" || comment.Text != input.Text {
		t.Fatalf("неверный комментарий: %+v", comment)
	}
	parentID = uuid.New().String()
	if comment.ParentID.String() == parentID {
		t.Fatal("ID разделяется с входом")
	}
	input.ParentID = nil
	root, err := input.ToDomain("actor", time.Now())
	if err != nil || !root.IsRoot() {
		t.Fatalf("корень: %v", err)
	}
	for _, invalid := range []string{"", "bad", uuid.Nil.String()} {
		input.ParentID = &invalid
		if _, err := input.ToDomain("actor", time.Now()); !errors.Is(err, domain.ErrInvalidID) {
			t.Fatalf("parentId=%q: %v", invalid, err)
		}
	}
	input.ParentID = nil
	input.PostID = "bad"
	if _, err := input.ToDomain("actor", time.Now()); !errors.Is(err, domain.ErrInvalidID) {
		t.Fatal(err)
	}
	input.PostID = postID
	input.Text = strings.Repeat("я", 2001)
	if _, err := input.ToDomain("actor", time.Now()); !errors.Is(err, domain.ErrCommentTooLong) {
		t.Fatal(err)
	}
}

func TestSetCommentsEnabledInput(t *testing.T) {
	input := SetCommentsEnabledInput{PostID: uuid.New().String(), Enabled: false}
	params, err := input.ToParams("actor")
	if err != nil {
		t.Fatal(err)
	}
	if params.PostID.String() != input.PostID || params.ActorID != "actor" || params.Enabled {
		t.Fatalf("неверные параметры: %+v", params)
	}
	if _, err := input.ToParams(" "); !errors.Is(err, domain.ErrEmptyAuthor) {
		t.Fatal(err)
	}
	input.PostID = ""
	if _, err := input.ToParams("actor"); !errors.Is(err, domain.ErrInvalidID) {
		t.Fatal(err)
	}
}

func TestParseID(t *testing.T) {
	id := uuid.New()
	for _, text := range []string{id.String(), strings.ToUpper(id.String())} {
		parsed, err := ParseID(text)
		if err != nil || parsed != id {
			t.Fatalf("UUID %q: %v", text, err)
		}
	}
	for _, text := range []string{"", "bad", uuid.Nil.String(), "urn:uuid:" + id.String(), strings.ReplaceAll(id.String(), "-", ""), strings.Repeat("z", 36)} {
		if _, err := ParseID(text); !errors.Is(err, domain.ErrInvalidID) {
			t.Fatalf("неверный UUID %q: %v", text, err)
		}
	}
}
