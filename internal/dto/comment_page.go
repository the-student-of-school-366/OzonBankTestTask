package dto

// RootPageInput — аргументы одной страницы корней в пакетном запросе.
type RootPageInput struct {
	PostID string `json:"postId"`
	PageInput
}

// ReplyPageInput — аргументы одной страницы ответов в пакетном запросе.
type ReplyPageInput struct {
	ParentID string `json:"parentId"`
	PageInput
}
