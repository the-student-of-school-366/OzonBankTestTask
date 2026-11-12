package comment

import (
	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/post"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewCommentText(t *testing.T) {
	postID := uuid.New()
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.FixedZone("MSK", 3*60*60))

	tests := []struct {
		name    string
		text    string
		wantErr error
	}{
		{name: "обычный текст", text: "первый комментарий"},
		{name: "ровно 2000 символов", text: strings.Repeat("я", MaxCommentLength)},
		{name: "2000 emoji", text: strings.Repeat("🙂", MaxCommentLength)},
		{name: "1000 комбинируемых символов", text: strings.Repeat("e\u0301", 1000)},
		{name: "сохраняемые пробелы", text: "  текст \n"},
		{name: "пустая строка", text: "", wantErr: domain.ErrEmptyCommentText},
		{name: "только пробелы", text: " \n\t", wantErr: domain.ErrEmptyCommentText},
		{name: "2001 символ", text: strings.Repeat("я", MaxCommentLength+1), wantErr: domain.ErrCommentTooLong},
		{name: "2001 emoji", text: strings.Repeat("🙂", MaxCommentLength+1), wantErr: domain.ErrCommentTooLong},
		{name: "пробел Unicode", text: "\u2003\u00a0", wantErr: domain.ErrEmptyCommentText},
		{name: "невалидный UTF-8", text: string([]byte{0xff}), wantErr: domain.ErrInvalidText},
		{name: "нулевой символ", text: "a\x00b", wantErr: domain.ErrInvalidText},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comment, err := NewComment(postID, nil, "анна", tt.text, now)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ошибка %v, ожидалась %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("неожиданная ошибка: %v", err)
			}
			if comment.ID == uuid.Nil {
				t.Fatal("идентификатор не назначен")
			}
			if comment.PostID != postID {
				t.Fatalf("post id: получили %s, ожидали %s", comment.PostID, postID)
			}
			if comment.Text != tt.text {
				t.Fatal("исходный текст изменён")
			}
			if !comment.IsRoot() {
				t.Fatal("комментарий без родителя должен быть корневым")
			}
			if !comment.CreatedAt.Equal(now.UTC()) {
				t.Fatalf("время %s, ожидали %s", comment.CreatedAt, now.UTC())
			}
		})
	}
}

func TestNewCommentCopiesParent(t *testing.T) {
	parentID := uuid.New()
	comment, err := NewComment(uuid.New(), &parentID, "анна", "ответ", time.Time{})
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if comment.IsRoot() || comment.ParentID == nil || *comment.ParentID != parentID {
		t.Fatalf("родитель: %+v", comment.ParentID)
	}

	parentID = uuid.New()
	if *comment.ParentID == parentID {
		t.Fatal("смена исходного id изменила комментарий")
	}

	nilID := uuid.Nil
	if _, err := NewComment(uuid.New(), &nilID, "анна", "корень", time.Time{}); !errors.Is(err, domain.ErrInvalidID) {
		t.Fatalf("нулевой parent id: %v", err)
	}
}

func TestNewPost(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	postItem, err := post.NewPost("заголовок", "текст", "петр", false, now)
	if err != nil {
		t.Fatal(err)
	}
	if postItem.ID == uuid.Nil {
		t.Fatal("идентификатор не назначен")
	}
	if postItem.Title != "заголовок" || postItem.Content != "текст" || postItem.Author != "петр" {
		t.Fatalf("поля поста: %+v", postItem)
	}
	if postItem.CommentsEnabled {
		t.Fatal("флаг комментариев должен сохраниться выключенным")
	}
	if !postItem.CreatedAt.Equal(now) {
		t.Fatalf("время %s", postItem.CreatedAt)
	}
}

func TestCommentValidationAndClone(t *testing.T) {
	parentID := uuid.New()
	comment, err := NewComment(uuid.New(), &parentID, "author", "reply", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	clone := comment.Clone()
	*clone.ParentID = uuid.New()
	if *comment.ParentID != parentID {
		t.Fatal("Clone разделяет ParentID с исходным комментарием")
	}
	tests := []struct {
		name   string
		change func(*Comment)
		want   error
	}{
		{"нулевой ID", func(c *Comment) { c.ID = uuid.Nil }, domain.ErrInvalidID},
		{"нулевой post ID", func(c *Comment) { c.PostID = uuid.Nil }, domain.ErrInvalidID},
		{"пустой автор", func(c *Comment) { c.Author = " " }, domain.ErrEmptyAuthor},
		{"NUL в авторе", func(c *Comment) { c.Author = "a\x00" }, domain.ErrInvalidText},
		{"ссылка на себя", func(c *Comment) { c.ParentID = &c.ID }, domain.ErrSelfParent},
		{"нулевое время", func(c *Comment) { c.CreatedAt = time.Time{} }, domain.ErrInvalidTime},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := comment.Clone()
			tt.change(&candidate)
			if err := candidate.Validate(); !errors.Is(err, tt.want) {
				t.Fatalf("ошибка %v, ожидалась %v", err, tt.want)
			}
		})
	}
}

func TestValidateCommentPlacement(t *testing.T) {
	p, err := post.NewPost("title", "content", "author", true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	parent, err := NewComment(p.ID, nil, "reader", "root", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	reply, err := NewComment(p.ID, &parent.ID, "reader", "reply", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	disabled := *p
	disabled.CommentsEnabled = false
	otherParent := *parent
	otherParent.PostID = uuid.New()
	wrongParent := *parent
	wrongParent.ID = uuid.New()
	otherPost := *p
	otherPost.ID = uuid.New()
	tests := []struct {
		name    string
		post    *post.Post
		parent  *Comment
		comment Comment
		want    error
	}{
		{"корень", p, nil, *parent, nil},
		{"ответ", p, parent, *reply, nil},
		{"нет поста", nil, nil, *parent, domain.ErrPostNotFound},
		{"другой пост", &otherPost, parent, *reply, domain.ErrPostNotFound},
		{"выключены корни", &disabled, nil, *parent, domain.ErrCommentsDisabled},
		{"выключены ответы", &disabled, parent, *reply, domain.ErrCommentsDisabled},
		{"нет родителя", p, nil, *reply, domain.ErrCommentNotFound},
		{"другой родитель", p, &wrongParent, *reply, domain.ErrCommentNotFound},
		{"родитель другого поста", p, &otherParent, *reply, domain.ErrParentPostMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateCommentPlacement(tt.post, tt.parent, tt.comment); !errors.Is(err, tt.want) {
				t.Fatalf("ошибка %v, ожидалась %v", err, tt.want)
			}
		})
	}
}

func TestDeepCommentChain(t *testing.T) {
	postItem, err := post.NewPost("title", "content", "author", true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var parent *Comment
	for i := 0; i < 1000; i++ {
		var parentID *uuid.UUID
		if parent != nil {
			parentID = &parent.ID
		}
		comment, err := NewComment(postItem.ID, parentID, "reader", "reply", time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateCommentPlacement(postItem, parent, *comment); err != nil {
			t.Fatal(err)
		}
		parent = comment
	}
}
