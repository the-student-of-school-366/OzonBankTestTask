package comment

import (
	"context"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/repositories/postgres"
	"OzonBankTestTask/internal/repositories/postgres/queries"
	commentusecase "OzonBankTestTask/internal/usecase/comment"

	"github.com/google/uuid"
)

func (r *Repository) ListRootsBatch(ctx context.Context, requests []commentusecase.RootPageRequest) ([]page.Page[comment.Comment], error) {
	ids, pages := make([]uuid.UUID, len(requests)), make([]page.Request, len(requests))
	for i, request := range requests {
		ids[i], pages[i] = request.PostID, request.Page
	}
	return r.listPages(ctx, ids, pages, queries.RootsBatch, domain.ErrPostNotFound)
}

func (r *Repository) ListRepliesBatch(ctx context.Context, requests []commentusecase.ReplyPageRequest) ([]page.Page[comment.Comment], error) {
	ids, pages := make([]uuid.UUID, len(requests)), make([]page.Request, len(requests))
	for i, request := range requests {
		ids[i], pages[i] = request.ParentID, request.Page
	}
	return r.listPages(ctx, ids, pages, queries.RepliesBatch, domain.ErrCommentNotFound)
}

func (r *Repository) listPages(ctx context.Context, ids []uuid.UUID, pages []page.Request, query string, missing error) ([]page.Page[comment.Comment], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	args, err := postgres.PageParameters(ids, pages)
	if err != nil {
		return nil, err
	}
	result := make([]page.Page[comment.Comment], len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, postgres.Error("list comment pages", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ordinal int64
		var exists bool
		var record postgres.CommentRow
		if err = rows.Scan(append([]any{&ordinal, &exists}, record.Destinations()...)...); err != nil {
			return nil, postgres.Error("scan comment page", err)
		}
		if !exists {
			return nil, missing
		}
		if ordinal < 1 || ordinal > int64(len(result)) {
			return nil, domain.ErrStorage
		}
		if record.ID.Valid {
			value, err := record.Value()
			if err != nil {
				return nil, postgres.Error("scan comment", err)
			}
			result[ordinal-1].Items = append(result[ordinal-1].Items, value)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.Error("list comment pages", err)
	}
	for i := range result {
		result[i], err = page.CutPage(result[i].Items, pages[i].First)
		if err != nil {
			return nil, postgres.Error("cut comment page", err)
		}
	}
	return result, nil
}
