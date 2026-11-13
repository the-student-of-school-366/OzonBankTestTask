package queries

import "embed"

const (
	PostCreate      = "post_create"
	PostsByID       = "posts_by_id"
	PostsFirstPage  = "posts_first_page"
	PostsNextPage   = "posts_next_page"
	PostForShare    = "post_for_share"
	PostForUpdate   = "post_for_update"
	PostSetComments = "post_set_comments"
	CommentCreate   = "comment_create"
	CommentByID     = "comment_by_id"
	CommentsByID    = "comments_by_id"
	RootsBatch      = "roots_batch"
	RepliesBatch    = "replies_batch"
)

//go:embed *.sql
var files embed.FS

type Statement struct{ Name, SQL string }

// All возвращает независимый реестр запросов для подготовки каждого соединения.
func All() []Statement {
	names := []string{PostCreate, PostsByID, PostsFirstPage, PostsNextPage, PostForShare, PostForUpdate, PostSetComments, CommentCreate, CommentByID, CommentsByID, RootsBatch, RepliesBatch}
	result := make([]Statement, len(names))
	for i, name := range names {
		data, err := files.ReadFile(name + ".sql")
		if err != nil {
			panic(err)
		} // Файлы являются частью бинарника.
		result[i] = Statement{Name: name, SQL: string(data)}
	}
	return result
}
