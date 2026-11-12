package comment

import (
	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/domain/post"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// MaxCommentLength — верхняя граница текста комментария в символах Unicode.
const MaxCommentLength = 2000

// Comment — узел дерева обсуждения.
// ParentID == nil означает корневой комментарий поста.
// Вложенность не ограничена: ответ хранится такой же записью со своим родителем.
type Comment struct {
	ID        uuid.UUID
	PostID    uuid.UUID
	ParentID  *uuid.UUID
	Author    string // Стабильный ID автора.
	Text      string
	CreatedAt time.Time
}

// NewComment создаёт комментарий с новым идентификатором.
// Только nil parentID означает корень; нулевой UUID отклоняется.
func NewComment(postID uuid.UUID, parentID *uuid.UUID, author, text string, now time.Time) (*Comment, error) {
	var parent *uuid.UUID
	if parentID != nil {
		id := *parentID
		parent = &id
	}

	comment := &Comment{
		ID:        uuid.New(),
		PostID:    postID,
		ParentID:  parent,
		Author:    author,
		Text:      text,
		CreatedAt: domain.NormalizeTime(now),
	}
	if err := comment.Validate(); err != nil {
		return nil, err
	}
	return comment, nil
}

func (c Comment) Validate() error {
	if c.ID == uuid.Nil || c.PostID == uuid.Nil || (c.ParentID != nil && *c.ParentID == uuid.Nil) {
		return domain.ErrInvalidID
	}
	if c.ParentID != nil && *c.ParentID == c.ID {
		return domain.ErrSelfParent
	}
	if err := domain.ValidateAuthor(c.Author); err != nil {
		return err
	}
	if err := ValidateCommentText(c.Text); err != nil {
		return err
	}
	return domain.ValidateTime(c.CreatedAt)
}

// IsRoot сообщает, что комментарий оставлен прямо под постом, а не в ответ.
func (c Comment) IsRoot() bool {
	return c.ParentID == nil
}

// ValidateCommentText проверяет инвариант длины.
func ValidateCommentText(text string) error {
	if err := domain.ValidateRequiredText(text, domain.ErrEmptyCommentText); err != nil {
		return err
	}
	if utf8.RuneCountInString(text) > MaxCommentLength {
		return domain.ErrCommentTooLong
	}
	return nil
}

// Clone возвращает копию, не разделяющую ParentID с исходным объектом.
func (c Comment) Clone() Comment {
	if c.ParentID != nil {
		id := *c.ParentID
		c.ParentID = &id
	}
	return c
}

func (c Comment) PagePosition() page.Position {
	return page.Position{CreatedAt: c.CreatedAt, ID: c.ID}
}

// ValidateCommentPlacement проверяет связи на согласованном состоянии хранилища.
// Проверка и последующая вставка должны выполняться в одной критической секции.
// Родитель нужен только для ответа; nil означает, что он не найден.
func ValidateCommentPlacement(post *post.Post, parent *Comment, comment Comment) error {
	if err := comment.Validate(); err != nil {
		return err
	}
	if post == nil || post.ID != comment.PostID {
		return domain.ErrPostNotFound
	}
	if err := post.EnsureCommentsEnabled(); err != nil {
		return err
	}
	if comment.IsRoot() {
		return nil
	}
	if parent == nil || parent.ID != *comment.ParentID {
		return domain.ErrCommentNotFound
	}
	if parent.PostID != comment.PostID {
		return domain.ErrParentPostMismatch
	}
	return nil
}
