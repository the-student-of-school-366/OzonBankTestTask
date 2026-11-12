package post

import (
	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/page"
	"time"

	"github.com/google/uuid"
)

// Post — публикация, к которой оставляют комментарии.
type Post struct {
	ID              uuid.UUID
	Title           string
	Content         string
	Author          string // Стабильный ID автора, а не отображаемое имя.
	CommentsEnabled bool
	CreatedAt       time.Time
}

func NewPost(title, content, author string, commentsEnabled bool, now time.Time) (*Post, error) {
	post := &Post{
		ID:              uuid.New(),
		Title:           title,
		Content:         content,
		Author:          author,
		CommentsEnabled: commentsEnabled,
		CreatedAt:       domain.NormalizeTime(now),
	}
	if err := post.Validate(); err != nil {
		return nil, err
	}
	return post, nil
}

// Validate также применим к сущностям, восстановленным из хранилища.
func (p Post) Validate() error {
	if p.ID == uuid.Nil {
		return domain.ErrInvalidID
	}
	if err := domain.ValidateRequiredText(p.Title, domain.ErrEmptyPostTitle); err != nil {
		return err
	}
	if err := domain.ValidateRequiredText(p.Content, domain.ErrEmptyPostContent); err != nil {
		return err
	}
	if err := domain.ValidateAuthor(p.Author); err != nil {
		return err
	}
	return domain.ValidateTime(p.CreatedAt)
}

func (p Post) EnsureAuthor(actorID string) error {
	if err := domain.ValidateAuthor(actorID); err != nil {
		return err
	}
	if p.Author != actorID {
		return domain.ErrNotAuthor
	}
	return nil
}

func (p Post) EnsureCommentsEnabled() error {
	if !p.CommentsEnabled {
		return domain.ErrCommentsDisabled
	}
	return nil
}

// SetCommentsEnabled вызывается внутри блокировки/транзакции хранилища.
// Сам доменный объект синхронизацией не владеет.
func (p *Post) SetCommentsEnabled(actorID string, enabled bool) error {
	if err := p.EnsureAuthor(actorID); err != nil {
		return err
	}
	p.CommentsEnabled = enabled
	return nil
}

func (p Post) PagePosition() page.Position {
	return page.Position{CreatedAt: p.CreatedAt, ID: p.ID}
}
