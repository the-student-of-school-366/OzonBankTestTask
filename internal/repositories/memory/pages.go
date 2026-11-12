package memory

import (
	"context"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/domain/page"
	commentusecase "OzonBankTestTask/internal/usecase/comment"

	"github.com/google/uuid"
)

func (s *Store) ListRootsBatch(ctx context.Context, requests []commentusecase.RootPageRequest) ([]page.Page[comment.Comment], error) {
	ids, pages := make([]uuid.UUID, len(requests)), make([]page.Request, len(requests))
	for i, request := range requests {
		ids[i], pages[i] = request.PostID, request.Page
	}
	return s.listPages(ctx, ids, pages, true)
}

func (s *Store) ListRepliesBatch(ctx context.Context, requests []commentusecase.ReplyPageRequest) ([]page.Page[comment.Comment], error) {
	ids, pages := make([]uuid.UUID, len(requests)), make([]page.Request, len(requests))
	for i, request := range requests {
		ids[i], pages[i] = request.ParentID, request.Page
	}
	return s.listPages(ctx, ids, pages, false)
}

func (s *Store) listPages(ctx context.Context, ids []uuid.UUID, pages []page.Request, roots bool) ([]page.Page[comment.Comment], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateIDs(ids); err != nil {
		return nil, err
	}
	for i, request := range pages {
		valid, err := page.NewPageRequest(request.First, request.After)
		if err != nil {
			return nil, err
		}
		pages[i] = valid
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := make([]page.Page[comment.Comment], len(ids))
	for i, id := range ids {
		var index []page.Position
		if roots {
			if _, ok := s.posts[id]; !ok {
				return nil, domain.ErrPostNotFound
			}
			index = s.roots[id]
		} else {
			if _, ok := s.comments[id]; !ok {
				return nil, domain.ErrCommentNotFound
			}
			index = s.replies[id]
		}
		positions := positionsForPage(index, pages[i])
		values := make([]comment.Comment, 0, len(positions))
		for _, pos := range positions {
			values = append(values, s.comments[pos.ID].Clone())
		}
		var err error
		result[i], err = page.CutPage(values, pages[i].First)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
