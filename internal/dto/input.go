package dto

import (
	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/domain/post"
	"fmt"
	"time"

	"OzonBankTestTask/internal/domain"

	"github.com/google/uuid"
)

type CreatePostInput struct {
	Title           string `json:"title"`
	Content         string `json:"content"`
	CommentsEnabled *bool  `json:"commentsEnabled,omitempty"`
}

// ToDomain сохраняет явное false; отсутствие значения означает true.
func (input CreatePostInput) ToDomain(actorID string, now time.Time) (*post.Post, error) {
	enabled := true
	if input.CommentsEnabled != nil {
		enabled = *input.CommentsEnabled
	}
	return post.NewPost(input.Title, input.Content, actorID, enabled, now)
}

type CreateCommentInput struct {
	PostID   string  `json:"postId"`
	ParentID *string `json:"parentId,omitempty"`
	Text     string  `json:"text"`
}

func (input CreateCommentInput) ToDomain(actorID string, now time.Time) (*comment.Comment, error) {
	postID, err := ParseID(input.PostID)
	if err != nil {
		return nil, fmt.Errorf("postId: %w", err)
	}
	var parentID *uuid.UUID
	if input.ParentID != nil {
		id, err := ParseID(*input.ParentID)
		if err != nil {
			return nil, fmt.Errorf("parentId: %w", err)
		}
		parentID = &id
	}
	return comment.NewComment(postID, parentID, actorID, input.Text, now)
}

type SetCommentsEnabledInput struct {
	PostID  string `json:"postId"`
	Enabled bool   `json:"enabled"`
}

// SetCommentsEnabledParams — разобранные параметры для сценария изменения поста.
// Сравнение ActorID с автором поста выполняется внутри атомарной операции хранилища.
type SetCommentsEnabledParams struct {
	PostID  uuid.UUID
	ActorID string
	Enabled bool
}

func (input SetCommentsEnabledInput) ToParams(actorID string) (SetCommentsEnabledParams, error) {
	id, err := ParseID(input.PostID)
	if err != nil {
		return SetCommentsEnabledParams{}, fmt.Errorf("postId: %w", err)
	}
	if err := domain.ValidateAuthor(actorID); err != nil {
		return SetCommentsEnabledParams{}, err
	}
	return SetCommentsEnabledParams{PostID: id, ActorID: actorID, Enabled: input.Enabled}, nil
}

func ParseID(value string) (uuid.UUID, error) {
	if len(value) != 36 {
		return uuid.Nil, domain.ErrInvalidID
	}
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, domain.ErrInvalidID
	}
	return id, nil
}
