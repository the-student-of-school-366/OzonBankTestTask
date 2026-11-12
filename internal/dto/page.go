package dto

import (
	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/comment"
	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/domain/post"
	"OzonBankTestTask/internal/pagination"
)

// PageInput применяется после проверки GraphQL non-null аргументов схемой.
// nil First означает, что аргумент не задан; nil After — первую страницу.
type PageInput struct {
	First *int    `json:"first,omitempty"`
	After *string `json:"after,omitempty"`
}

func (input PageInput) ToRequest(scope pagination.Scope) (page.Request, error) {
	if err := scope.Validate(); err != nil {
		return page.Request{}, err
	}
	first := page.DefaultPageSize
	if input.First != nil {
		first = *input.First
	}
	request, err := page.NewPageRequest(first, nil)
	if err != nil || input.After == nil {
		return request, err
	}
	position, err := pagination.Decode(*input.After, scope)
	if err != nil {
		return page.Request{}, err
	}
	return page.NewPageRequest(first, &position)
}

type PageInfo struct {
	EndCursor   *string `json:"endCursor"`
	HasNextPage bool    `json:"hasNextPage"`
}

type PostConnection struct {
	Nodes    []Post   `json:"nodes"`
	PageInfo PageInfo `json:"pageInfo"`
}

type CommentConnection struct {
	Nodes    []Comment `json:"nodes"`
	PageInfo PageInfo  `json:"pageInfo"`
}

func PostConnectionFromDomain(page page.Page[post.Post]) (PostConnection, error) {
	info, err := pageInfo(page.End, page.HasNextPage, pagination.Scope{Kind: pagination.Posts})
	if err != nil {
		return PostConnection{}, err
	}
	nodes := make([]Post, len(page.Items))
	for i, postItem := range page.Items {
		nodes[i] = PostFromDomain(postItem)
	}
	return PostConnection{Nodes: nodes, PageInfo: info}, nil
}

func CommentConnectionFromDomain(page page.Page[comment.Comment], scope pagination.Scope) (CommentConnection, error) {
	if scope.Kind != pagination.Roots && scope.Kind != pagination.Replies {
		return CommentConnection{}, domain.ErrInvalidCursor
	}
	info, err := pageInfo(page.End, page.HasNextPage, scope)
	if err != nil {
		return CommentConnection{}, err
	}
	nodes := make([]Comment, len(page.Items))
	for i, commentItem := range page.Items {
		nodes[i] = CommentFromDomain(commentItem)
	}
	return CommentConnection{Nodes: nodes, PageInfo: info}, nil
}

func pageInfo(end *page.Position, hasNext bool, scope pagination.Scope) (PageInfo, error) {
	if err := scope.Validate(); err != nil {
		return PageInfo{}, err
	}
	info := PageInfo{HasNextPage: hasNext}
	if end != nil {
		cursor, err := pagination.Encode(*end, scope)
		if err != nil {
			return PageInfo{}, err
		}
		info.EndCursor = &cursor
	}
	return info, nil
}
