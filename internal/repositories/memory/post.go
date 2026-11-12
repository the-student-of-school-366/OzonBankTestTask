package memory

import (
	"context"
	"fmt"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/domain/post"

	"github.com/google/uuid"
)

func (s *Store) CreatePost(ctx context.Context, value post.Post) (post.Post, error) {
	if err := ctx.Err(); err != nil {
		return post.Post{}, err
	}
	if err := value.Validate(); err != nil {
		return post.Post{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return post.Post{}, err
	}
	if _, exists := s.posts[value.ID]; exists {
		return post.Post{}, fmt.Errorf("%w: duplicate post ID", domain.ErrStorage)
	}
	value.CreatedAt = value.CreatedAt.UTC()
	s.posts[value.ID] = value
	s.postOrder = insertPosition(s.postOrder, value.PagePosition())
	return value, nil
}

func (s *Store) PostsByID(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]post.Post, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateIDs(ids); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := make(map[uuid.UUID]post.Post, len(ids))
	for _, id := range ids {
		if value, ok := s.posts[id]; ok {
			result[id] = value
		}
	}
	return result, nil
}

func (s *Store) ListPosts(ctx context.Context, request page.Request) (page.Page[post.Post], error) {
	if err := ctx.Err(); err != nil {
		return page.Page[post.Post]{}, err
	}
	request, err := page.NewPageRequest(request.First, request.After)
	if err != nil {
		return page.Page[post.Post]{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return page.Page[post.Post]{}, err
	}
	positions := positionsForPage(s.postOrder, request)
	values := make([]post.Post, 0, len(positions))
	for _, pos := range positions {
		values = append(values, s.posts[pos.ID])
	}
	return page.CutPage(values, request.First)
}

func (s *Store) SetCommentsEnabled(ctx context.Context, id uuid.UUID, actorID string, enabled bool) (post.Post, error) {
	if err := ctx.Err(); err != nil {
		return post.Post{}, err
	}
	if id == uuid.Nil {
		return post.Post{}, domain.ErrInvalidID
	}
	if err := domain.ValidateAuthor(actorID); err != nil {
		return post.Post{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return post.Post{}, err
	}
	value, ok := s.posts[id]
	if !ok {
		return post.Post{}, domain.ErrPostNotFound
	}
	if err := value.SetCommentsEnabled(actorID, enabled); err != nil {
		return post.Post{}, err
	}
	s.posts[id] = value
	return value, nil
}
