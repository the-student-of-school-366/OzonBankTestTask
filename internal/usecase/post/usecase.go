package post

import (
	"context"
	"fmt"
	"time"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/dto"
	"OzonBankTestTask/internal/pagination"
	"OzonBankTestTask/internal/usecase"
)

type UseCase struct {
	repository Repository
	now        func() time.Time
}

// New принимает обязательный репозиторий; конкретный backend выбирается в main.
func New(repository Repository, opts ...usecase.Option) *UseCase {
	return &UseCase{repository: repository, now: usecase.ResolveClock(opts...)}
}

func (u *UseCase) Create(ctx context.Context, actorID string, input dto.CreatePostInput) (dto.Post, error) {
	if err := ctx.Err(); err != nil {
		return dto.Post{}, err
	}
	value, err := input.ToDomain(actorID, u.now())
	if err != nil {
		return dto.Post{}, fmt.Errorf("создание поста: %w", err)
	}
	saved, err := u.repository.CreatePost(ctx, *value)
	if err != nil {
		return dto.Post{}, usecase.StorageError(ctx, "создание поста", err)
	}
	return dto.PostFromDomain(saved), nil
}

// Get возвращает nil без ошибки, если корректный ID не найден.
func (u *UseCase) Get(ctx context.Context, id string) (*dto.Post, error) {
	values, err := u.GetMany(ctx, []string{id})
	if err != nil {
		return nil, err
	}
	return values[0], nil
}

// GetMany сохраняет порядок и повторы ID; отсутствующим постам соответствуют nil.
func (u *UseCase) GetMany(ctx context.Context, ids []string) ([]*dto.Post, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	parsed, err := usecase.ParseIDs(ids)
	if err != nil {
		return nil, err
	}
	values, err := usecase.LoadByIDs(ctx, parsed, u.repository.PostsByID)
	if err != nil {
		return nil, usecase.StorageError(ctx, "чтение постов", err)
	}
	result := make([]*dto.Post, len(parsed))
	for i, id := range parsed {
		if value, exists := values[id]; exists {
			converted := dto.PostFromDomain(value)
			result[i] = &converted
		}
	}
	return result, nil
}

func (u *UseCase) List(ctx context.Context, input dto.PageInput) (dto.PostConnection, error) {
	if err := ctx.Err(); err != nil {
		return dto.PostConnection{}, err
	}
	request, err := input.ToRequest(pagination.Scope{Kind: pagination.Posts})
	if err != nil {
		return dto.PostConnection{}, err
	}
	values, err := u.repository.ListPosts(ctx, request)
	if err != nil {
		return dto.PostConnection{}, usecase.StorageError(ctx, "список постов", err)
	}
	result, err := dto.PostConnectionFromDomain(values)
	if err != nil {
		return dto.PostConnection{}, fmt.Errorf("%w: формирование страницы постов: %v", domain.ErrStorage, err)
	}
	return result, nil
}

func (u *UseCase) SetCommentsEnabled(ctx context.Context, actorID string, input dto.SetCommentsEnabledInput) (dto.Post, error) {
	if err := ctx.Err(); err != nil {
		return dto.Post{}, err
	}
	params, err := input.ToParams(actorID)
	if err != nil {
		return dto.Post{}, err
	}
	// Не читать пост перед обновлением: проверка автора принадлежит атомарному методу.
	saved, err := u.repository.SetCommentsEnabled(ctx, params.PostID, params.ActorID, params.Enabled)
	if err != nil {
		return dto.Post{}, usecase.StorageError(ctx, "изменение настроек комментариев", err)
	}
	return dto.PostFromDomain(saved), nil
}
