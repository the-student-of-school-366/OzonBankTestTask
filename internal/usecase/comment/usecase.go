package comment

import (
	"context"
	"fmt"
	"time"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/dto"
	"OzonBankTestTask/internal/pagination"
	"OzonBankTestTask/internal/usecase"
)

type UseCase struct {
	repository Repository
	publisher  Publisher
	now        func() time.Time
}

func New(repository Repository, publisher Publisher, opts ...usecase.Option) *UseCase {
	return &UseCase{repository: repository, publisher: publisher, now: usecase.ResolveClock(opts...)}
}

func (u *UseCase) Create(ctx context.Context, actorID string, input dto.CreateCommentInput) (dto.Comment, error) {
	if err := ctx.Err(); err != nil {
		return dto.Comment{}, err
	}
	value, err := input.ToDomain(actorID, u.now())
	if err != nil {
		return dto.Comment{}, fmt.Errorf("создание комментария: %w", err)
	}
	saved, err := u.repository.CreateComment(ctx, *value)
	if err != nil {
		return dto.Comment{}, usecase.StorageError(ctx, "создание комментария", err)
	}
	result := dto.CommentFromDomain(saved)
	if u.publisher != nil {
		u.publisher.Publish(saved.Clone())
	}
	return result, nil
}

// Get возвращает nil без ошибки, если корректный ID не найден.
func (u *UseCase) Get(ctx context.Context, id string) (*dto.Comment, error) {
	values, err := u.GetMany(ctx, []string{id})
	if err != nil {
		return nil, err
	}
	return values[0], nil
}

func (u *UseCase) GetMany(ctx context.Context, ids []string) ([]*dto.Comment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	parsed, err := usecase.ParseIDs(ids)
	if err != nil {
		return nil, err
	}
	values, err := usecase.LoadByIDs(ctx, parsed, u.repository.CommentsByID)
	if err != nil {
		return nil, usecase.StorageError(ctx, "чтение комментариев", err)
	}
	result := make([]*dto.Comment, len(parsed))
	for i, id := range parsed {
		if value, exists := values[id]; exists {
			converted := dto.CommentFromDomain(value)
			result[i] = &converted
		}
	}
	return result, nil
}

func (u *UseCase) ListRoots(ctx context.Context, postID string, input dto.PageInput) (dto.CommentConnection, error) {
	values, err := u.ListRootsBatch(ctx, []dto.RootPageInput{{PostID: postID, PageInput: input}})
	if err != nil {
		return dto.CommentConnection{}, err
	}
	return values[0], nil
}

func (u *UseCase) ListReplies(ctx context.Context, parentID string, input dto.PageInput) (dto.CommentConnection, error) {
	values, err := u.ListRepliesBatch(ctx, []dto.ReplyPageInput{{ParentID: parentID, PageInput: input}})
	if err != nil {
		return dto.CommentConnection{}, err
	}
	return values[0], nil
}

// ListRootsBatch проверяет весь ввод до первого обращения к репозиторию.
// Разные first/after одного поста остаются разными запросами.
func (u *UseCase) ListRootsBatch(ctx context.Context, inputs []dto.RootPageInput) ([]dto.CommentConnection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	requests := make([]RootPageRequest, len(inputs))
	scopes := make([]pagination.Scope, len(inputs))
	for i, input := range inputs {
		id, err := dto.ParseID(input.PostID)
		if err != nil {
			return nil, fmt.Errorf("postId[%d]: %w", i, err)
		}
		scopes[i] = pagination.Scope{Kind: pagination.Roots, ID: id}
		request, err := input.PageInput.ToRequest(scopes[i])
		if err != nil {
			return nil, fmt.Errorf("страница корней[%d]: %w", i, err)
		}
		requests[i] = RootPageRequest{PostID: id, Page: request}
	}
	values, err := usecase.LoadPages(ctx, requests, u.repository.ListRootsBatch)
	if err != nil {
		return nil, usecase.StorageError(ctx, "чтение корней", err)
	}
	return commentPages(values, scopes)
}

func (u *UseCase) ListRepliesBatch(ctx context.Context, inputs []dto.ReplyPageInput) ([]dto.CommentConnection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	requests := make([]ReplyPageRequest, len(inputs))
	scopes := make([]pagination.Scope, len(inputs))
	for i, input := range inputs {
		id, err := dto.ParseID(input.ParentID)
		if err != nil {
			return nil, fmt.Errorf("parentId[%d]: %w", i, err)
		}
		scopes[i] = pagination.Scope{Kind: pagination.Replies, ID: id}
		request, err := input.PageInput.ToRequest(scopes[i])
		if err != nil {
			return nil, fmt.Errorf("страница ответов[%d]: %w", i, err)
		}
		requests[i] = ReplyPageRequest{ParentID: id, Page: request}
	}
	values, err := usecase.LoadPages(ctx, requests, u.repository.ListRepliesBatch)
	if err != nil {
		return nil, usecase.StorageError(ctx, "чтение ответов", err)
	}
	return commentPages(values, scopes)
}

func commentPages(values []page.Page[comment.Comment], scopes []pagination.Scope) ([]dto.CommentConnection, error) {
	result := make([]dto.CommentConnection, len(values))
	for i, value := range values {
		converted, err := dto.CommentConnectionFromDomain(value, scopes[i])
		if err != nil {
			return nil, fmt.Errorf("%w: формирование страницы комментариев: %v", domain.ErrStorage, err)
		}
		result[i] = converted
	}
	return result, nil
}
