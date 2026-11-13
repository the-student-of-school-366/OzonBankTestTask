package memory

import (
	"context"
	"fmt"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/comment"

	"github.com/google/uuid"
)

func (s *Store) CreateComment(ctx context.Context, value comment.Comment) (comment.Comment, error) {
	if err := ctx.Err(); err != nil {
		return comment.Comment{}, err
	}
	if err := value.Validate(); err != nil {
		return comment.Comment{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return comment.Comment{}, err
	}
	post, postExists := s.posts[value.PostID]
	var parent *comment.Comment
	if value.ParentID != nil {
		if found, ok := s.comments[*value.ParentID]; ok {
			parent = &found
		}
	}
	if !postExists {
		return comment.Comment{}, domain.ErrPostNotFound
	}
	if err := comment.ValidateCommentPlacement(&post, parent, value); err != nil {
		return comment.Comment{}, err
	}
	if _, exists := s.comments[value.ID]; exists {
		return comment.Comment{}, fmt.Errorf("%w: duplicate comment ID", domain.ErrStorage)
	}
	value = value.Clone()
	value.CreatedAt = value.CreatedAt.UTC()
	s.comments[value.ID] = value
	if value.ParentID == nil {
		s.roots[value.PostID] = insertPosition(s.roots[value.PostID], value.PagePosition())
	} else {
		id := *value.ParentID
		s.replies[id] = insertPosition(s.replies[id], value.PagePosition())
	}
	return value.Clone(), nil
}

func (s *Store) CommentsByID(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]comment.Comment, error) {
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
	result := make(map[uuid.UUID]comment.Comment, len(ids))
	for _, id := range ids {
		if value, ok := s.comments[id]; ok {
			result[id] = value.Clone()
		}
	}
	return result, nil
}
