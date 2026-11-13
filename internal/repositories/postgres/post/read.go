package post

import (
	"context"

	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/domain/post"
	"OzonBankTestTask/internal/repositories/postgres"
	"OzonBankTestTask/internal/repositories/postgres/queries"

	"github.com/google/uuid"
)

func (r *Repository) PostsByID(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]post.Post, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	args, err := postgres.IDs(ids)
	if err != nil {
		return nil, err
	}
	result := make(map[uuid.UUID]post.Post, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := r.pool.Query(ctx, queries.PostsByID, args)
	if err != nil {
		return nil, postgres.Error("read posts", err)
	}
	defer rows.Close()
	for rows.Next() {
		value, err := postgres.ScanPost(rows)
		if err != nil {
			return nil, postgres.Error("scan post", err)
		}
		result[value.ID] = value
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.Error("read posts", err)
	}
	return result, nil
}

func (r *Repository) ListPosts(ctx context.Context, request page.Request) (page.Page[post.Post], error) {
	if err := ctx.Err(); err != nil {
		return page.Page[post.Post]{}, err
	}
	if _, err := page.NewPageRequest(request.First, request.After); err != nil {
		return page.Page[post.Post]{}, err
	}
	query, args := queries.PostsFirstPage, []any{request.First + 1}
	if request.After != nil {
		query, args = queries.PostsNextPage, []any{request.After.CreatedAt.UTC(), postgres.UUID(request.After.ID), request.First + 1}
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return page.Page[post.Post]{}, postgres.Error("list posts", err)
	}
	defer rows.Close()
	values := make([]post.Post, 0, request.First+1)
	for rows.Next() {
		value, err := postgres.ScanPost(rows)
		if err != nil {
			return page.Page[post.Post]{}, postgres.Error("scan post", err)
		}
		values = append(values, value)
	}
	if err = rows.Err(); err != nil {
		return page.Page[post.Post]{}, postgres.Error("list posts", err)
	}
	result, err := page.CutPage(values, request.First)
	return result, postgres.Error("cut posts page", err)
}
