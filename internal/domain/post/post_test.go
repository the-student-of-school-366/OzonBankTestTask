package post_test

import (
	"errors"
	"testing"
	"time"

	"OzonBankTestTask/internal/domain"
	comment2 "OzonBankTestTask/internal/domain/comment"
	postdomain "OzonBankTestTask/internal/domain/post"

	"github.com/google/uuid"
)

func TestPostValidation(t *testing.T) {
	tests := []struct {
		name, title, content, author string
		want                         error
	}{
		{"пустой заголовок", " \n", "content", "author", domain.ErrEmptyPostTitle},
		{"пустое содержание", "title", "", "author", domain.ErrEmptyPostContent},
		{"пустой автор", "title", "content", "\t", domain.ErrEmptyAuthor},
		{"NUL в заголовке", "a\x00", "content", "author", domain.ErrInvalidText},
		{"UTF-8 содержания", "title", string([]byte{0xff}), "author", domain.ErrInvalidText},
		{"NUL в авторе", "title", "content", "a\x00", domain.ErrInvalidText},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			post, err := postdomain.NewPost(tt.title, tt.content, tt.author, true, time.Now())
			if post != nil || !errors.Is(err, tt.want) {
				t.Fatalf("post=%v, ошибка %v, ожидалась %v", post, err, tt.want)
			}
		})
	}
	post, err := postdomain.NewPost(" title ", " content ", "author", true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if post.Title != " title " || post.Content != " content " {
		t.Fatal("изменены исходные строки")
	}
	post.ID = uuid.Nil
	if !errors.Is(post.Validate(), domain.ErrInvalidID) {
		t.Fatal("принят нулевой ID")
	}
}

func TestSetCommentsEnabled(t *testing.T) {
	post, err := postdomain.NewPost("title", "content", "author", true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := post.SetCommentsEnabled("other", false); !errors.Is(err, domain.ErrNotAuthor) {
		t.Fatalf("чужой автор: %v", err)
	}
	if !post.CommentsEnabled {
		t.Fatal("запрещённая операция изменила пост")
	}
	if err := post.SetCommentsEnabled(" ", false); !errors.Is(err, domain.ErrEmptyAuthor) {
		t.Fatal(err)
	}
	for _, enabled := range []bool{false, false, true} {
		if err := post.SetCommentsEnabled("author", enabled); err != nil {
			t.Fatal(err)
		}
		if post.CommentsEnabled != enabled {
			t.Fatal("флаг не изменён")
		}
	}
}

func TestCreationTimePrecision(t *testing.T) {
	moment := time.Date(2026, 11, 12, 10, 11, 12, 123456789, time.FixedZone("MSK", 3*60*60))
	post, err := postdomain.NewPost("title", "content", "author", true, moment)
	if err != nil {
		t.Fatal(err)
	}
	comment, err := comment2.NewComment(post.ID, nil, "author", "text", moment)
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range []time.Time{post.CreatedAt, comment.CreatedAt} {
		if got.Location() != time.UTC || !got.Equal(moment.Truncate(time.Microsecond)) {
			t.Fatalf("ненормализованное время: %v", got)
		}
	}
	before := time.Now().UTC().Truncate(time.Microsecond)
	automatic, err := postdomain.NewPost("title", "content", "author", true, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if automatic.CreatedAt.Before(before) || automatic.CreatedAt.After(time.Now()) {
		t.Fatal("неверное текущее время")
	}
	if _, err := postdomain.NewPost("title", "content", "author", true, time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)); !errors.Is(err, domain.ErrInvalidTime) {
		t.Fatalf("неверный год принят: %v", err)
	}
}
