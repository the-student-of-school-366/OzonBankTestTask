package comment

import (
	"context"

	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/domain/page"

	"github.com/google/uuid"
)

//go:generate go tool mockery --case=snake --all

type RootPageRequest struct {
	PostID uuid.UUID
	Page   page.Request
}

type ReplyPageRequest struct {
	ParentID uuid.UUID
	Page     page.Request
}

// Repository возвращает независимые копии данных, включая ParentID.
// Страницы отсортированы по (created_at DESC, id DESC).
type Repository interface {
	CreateComment(ctx context.Context, value comment.Comment) (comment.Comment, error)
	CommentsByID(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]comment.Comment, error)
	ListRootsBatch(ctx context.Context, requests []RootPageRequest) ([]page.Page[comment.Comment], error)
	ListRepliesBatch(ctx context.Context, requests []ReplyPageRequest) ([]page.Page[comment.Comment], error)
}

// Publisher — необязательная доставка событий внутри процесса.
type Publisher interface {
	Publish(value comment.Comment)
}
