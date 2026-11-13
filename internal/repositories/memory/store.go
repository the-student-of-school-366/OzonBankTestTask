package memory

import (
	"sync"

	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/domain/post"
	commentusecase "OzonBankTestTask/internal/usecase/comment"
	postusecase "OzonBankTestTask/internal/usecase/post"

	"github.com/google/uuid"
)

// Store хранит взаимосвязанные посты и комментарии под общей блокировкой.
// Создавайте один экземпляр и передавайте его обоим usecase.
type Store struct {
	mu        sync.RWMutex
	posts     map[uuid.UUID]post.Post
	comments  map[uuid.UUID]comment.Comment
	postOrder []page.Position
	roots     map[uuid.UUID][]page.Position
	replies   map[uuid.UUID][]page.Position
}

func New() *Store {
	return &Store{
		posts: make(map[uuid.UUID]post.Post), comments: make(map[uuid.UUID]comment.Comment),
		roots: make(map[uuid.UUID][]page.Position), replies: make(map[uuid.UUID][]page.Position),
	}
}

var _ postusecase.Repository = (*Store)(nil)
var _ commentusecase.Repository = (*Store)(nil)
