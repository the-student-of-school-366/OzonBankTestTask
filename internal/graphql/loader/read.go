package loader

import (
	"context"

	"OzonBankTestTask/internal/dto"
	"OzonBankTestTask/internal/pagination"
)

func (l *Loaders) Post(ctx context.Context, id string) (*dto.Post, error) {
	parsed, err := dto.ParseID(id)
	if err != nil {
		return nil, err
	}
	return l.posts.Load(ctx, parsed.String())
}

func (l *Loaders) Comment(ctx context.Context, id string) (*dto.Comment, error) {
	parsed, err := dto.ParseID(id)
	if err != nil {
		return nil, err
	}
	return l.comments.Load(ctx, parsed.String())
}

func (l *Loaders) Roots(ctx context.Context, postID string, first int, after *string) (dto.CommentConnection, error) {
	key, err := validatedPageKey(postID, first, after, pagination.Roots)
	if err != nil {
		return dto.CommentConnection{}, err
	}
	return l.roots.Load(ctx, key)
}

func (l *Loaders) Replies(ctx context.Context, parentID string, first int, after *string) (dto.CommentConnection, error) {
	key, err := validatedPageKey(parentID, first, after, pagination.Replies)
	if err != nil {
		return dto.CommentConnection{}, err
	}
	return l.replies.Load(ctx, key)
}
