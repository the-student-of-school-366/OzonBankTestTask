package comment_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/dto"
	"OzonBankTestTask/internal/usecase"
	commentusecase "OzonBankTestTask/internal/usecase/comment"

	"github.com/google/uuid"
)

func TestCommentPublishesSavedValueAfterCommitDespiteCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	postID, parentID := uuid.New(), uuid.New()
	parentString := parentID.String()
	saved := sampleComment(t, postID, &parentID)
	committed, publications := false, 0
	repo := commentRepositoryStub{create: func(gotCtx context.Context, value comment.Comment) (comment.Comment, error) {
		if gotCtx != ctx {
			t.Fatal("контекст запроса не передан хранилищу")
		}
		if value.PostID != postID || value.ParentID == nil || *value.ParentID != parentID || value.Author != "actor" || value.Text != "reply" || !value.CreatedAt.Equal(fixedClock()) {
			t.Fatalf("данные записи: %+v", value)
		}
		committed = true
		cancel() // Клиент отключился, но запись уже подтверждена.
		return saved, nil
	}}
	publisher := publisherFunc(func(value comment.Comment) {
		publications++
		if !committed || value.ID != saved.ID || !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatal("публикация должна идти после commit с сохранённым значением")
		}
		*value.ParentID = uuid.Nil
	})
	u := commentusecase.New(repo, publisher, usecase.WithClock(fixedClock))
	result, err := u.Create(ctx, "actor", dto.CreateCommentInput{PostID: postID.String(), ParentID: &parentString, Text: "reply"})
	if err != nil {
		t.Fatal(err)
	}
	if publications != 1 || result.ID != saved.ID.String() || result.ParentID == nil || *result.ParentID != parentString || *saved.ParentID != parentID {
		t.Fatalf("событие изменило сохранённый комментарий или результат: %+v", result)
	}
}

func TestCommentCreateWithoutPublisher(t *testing.T) {
	repo := commentRepositoryStub{create: func(_ context.Context, value comment.Comment) (comment.Comment, error) { return value, nil }}
	result, err := commentusecase.New(repo, nil).Create(context.Background(), "actor", dto.CreateCommentInput{PostID: uuid.NewString(), Text: "root"})
	if err != nil || result.ID == "" || result.ParentID != nil {
		t.Fatalf("создание без подписок: %+v, %v", result, err)
	}
}

func TestCommentErrorsDoNotPublish(t *testing.T) {
	broken := errors.New("commit failed")
	for _, want := range []error{domain.ErrPostNotFound, domain.ErrCommentNotFound, domain.ErrCommentsDisabled, domain.ErrParentPostMismatch, context.Canceled, context.DeadlineExceeded, broken} {
		t.Run(want.Error(), func(t *testing.T) {
			repo := commentRepositoryStub{create: func(context.Context, comment.Comment) (comment.Comment, error) {
				return comment.Comment{}, fmt.Errorf("insert: %w", want)
			}}
			publisher := publisherFunc(func(comment.Comment) {
				t.Fatal("ошибка записи не должна порождать событие")
			})
			result, err := commentusecase.New(repo, publisher).Create(context.Background(), "actor", dto.CreateCommentInput{PostID: uuid.NewString(), Text: "text"})
			if !errors.Is(err, want) || result.ID != "" {
				t.Fatalf("ошибка: %v, результат: %+v", err, result)
			}
			if errors.Is(err, domain.ErrStorage) != (want == broken) {
				t.Fatalf("неверная классификация: %v", err)
			}
		})
	}
}

func TestCommentValidationBeforeStorage(t *testing.T) {
	u := commentusecase.New(commentRepositoryStub{}, publisherFunc(func(comment.Comment) { t.Fatal("неожиданная публикация") }))
	parent := "bad"
	for _, tt := range []struct {
		input dto.CreateCommentInput
		actor string
		want  error
	}{
		{dto.CreateCommentInput{PostID: "bad", Text: "text"}, "actor", domain.ErrInvalidID},
		{dto.CreateCommentInput{PostID: uuid.NewString(), ParentID: &parent, Text: "text"}, "actor", domain.ErrInvalidID},
		{dto.CreateCommentInput{PostID: uuid.NewString(), Text: " "}, "actor", domain.ErrEmptyCommentText},
		{dto.CreateCommentInput{PostID: uuid.NewString(), Text: strings.Repeat("я", 2001)}, "actor", domain.ErrCommentTooLong},
		{dto.CreateCommentInput{PostID: uuid.NewString(), Text: "text"}, "", domain.ErrEmptyAuthor},
	} {
		if _, err := u.Create(context.Background(), tt.actor, tt.input); !errors.Is(err, tt.want) {
			t.Fatalf("ожидалось %v, получено %v", tt.want, err)
		}
	}
	if _, err := u.GetMany(context.Background(), []string{uuid.NewString(), "bad"}); !errors.Is(err, domain.ErrInvalidID) {
		t.Fatal(err)
	}
}

func TestCommentGetManyAndGet(t *testing.T) {
	parentID := uuid.New()
	value := sampleComment(t, uuid.New(), &parentID)
	missing := uuid.New()
	calls := 0
	repo := commentRepositoryStub{byID: func(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]comment.Comment, error) {
		calls++
		return map[uuid.UUID]comment.Comment{value.ID: value}, nil
	}}
	u := commentusecase.New(repo, nil)
	result, err := u.GetMany(context.Background(), []string{value.ID.String(), missing.String(), value.ID.String()})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(result) != 3 || result[0].ID != value.ID.String() || result[1] != nil || result[2].ID != value.ID.String() {
		t.Fatalf("порядок: %+v", result)
	}
	*result[0].ParentID = "changed"
	if *result[2].ParentID != parentID.String() || *value.ParentID != parentID {
		t.Fatal("DTO разделяют parentId")
	}
	found, err := u.Get(context.Background(), value.ID.String())
	if err != nil || found == nil || found.ID != value.ID.String() {
		t.Fatalf("чтение узла: %+v, %v", found, err)
	}
	found, err = u.Get(context.Background(), missing.String())
	if err != nil || found != nil {
		t.Fatalf("отсутствующий узел: %+v, %v", found, err)
	}
	empty, err := commentusecase.New(commentRepositoryStub{}, nil).GetMany(context.Background(), nil)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("пустой batch: %v, %v", empty, err)
	}
	broken := errors.New("read failed")
	repo.byID = func(context.Context, []uuid.UUID) (map[uuid.UUID]comment.Comment, error) { return nil, broken }
	if _, err := commentusecase.New(repo, nil).Get(context.Background(), value.ID.String()); !errors.Is(err, broken) || !errors.Is(err, domain.ErrStorage) {
		t.Fatal(err)
	}
}
