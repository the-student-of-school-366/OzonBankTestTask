package dto

import (
	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/domain/post"
	"time"
)

// Post не содержит заранее загруженного дерева. Поле comments разрешается отдельно.
type Post struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Content         string    `json:"content"`
	AuthorID        string    `json:"authorId"`
	CommentsEnabled bool      `json:"commentsEnabled"`
	CreatedAt       time.Time `json:"createdAt"`
}

// Comment подходит и для чтения узла, и для события commentAdded.
// Ответы загружаются отдельно, поэтому глубина дерева не увеличивает размер DTO.
type Comment struct {
	ID        string    `json:"id"`
	PostID    string    `json:"postId"`
	ParentID  *string   `json:"parentId"`
	AuthorID  string    `json:"authorId"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
}

func PostFromDomain(post post.Post) Post {
	return Post{
		ID: post.ID.String(), Title: post.Title, Content: post.Content,
		AuthorID: post.Author, CommentsEnabled: post.CommentsEnabled,
		CreatedAt: post.CreatedAt.UTC(),
	}
}

func CommentFromDomain(comment comment.Comment) Comment {
	var parentID *string
	if comment.ParentID != nil {
		id := comment.ParentID.String()
		parentID = &id
	}
	return Comment{
		ID: comment.ID.String(), PostID: comment.PostID.String(), ParentID: parentID,
		AuthorID: comment.Author, Text: comment.Text, CreatedAt: comment.CreatedAt.UTC(),
	}
}
