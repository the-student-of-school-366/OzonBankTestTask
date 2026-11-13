package loader

import (
	"context"
	"fmt"
	"time"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/domain/page"
	"OzonBankTestTask/internal/dto"
	"OzonBankTestTask/internal/pagination"

	"github.com/vikstrous/dataloadgen"
)

// Ключ содержит значения аргументов, а не адреса first/after из DTO.
type pageKey struct {
	id       string
	first    int
	after    string
	hasAfter bool
}

func newPageKey(id string, first int, after *string) pageKey {
	key := pageKey{id: id, first: first}
	if after != nil {
		key.after, key.hasAfter = *after, true
	}
	return key
}

func validatedPageKey(id string, first int, after *string, kind pagination.Kind) (pageKey, error) {
	parsed, err := dto.ParseID(id)
	if err != nil {
		return pageKey{}, err
	}
	key := newPageKey(parsed.String(), first, after)
	_, err = key.input().ToRequest(pagination.Scope{Kind: kind, ID: parsed})
	return key, err
}

func (k pageKey) input() dto.PageInput {
	input := dto.PageInput{First: &k.first}
	if k.hasAfter {
		input.After = &k.after
	}
	return input
}

// Loaders объединяет чтения и кэширует их только в пределах одной операции.
type Loaders struct {
	posts    *dataloadgen.Loader[string, *dto.Post]
	comments *dataloadgen.Loader[string, *dto.Comment]
	roots    *dataloadgen.Loader[pageKey, dto.CommentConnection]
	replies  *dataloadgen.Loader[pageKey, dto.CommentConnection]
}

func New(posts PostReader, comments CommentReader) *Loaders {
	return &Loaders{
		posts:    newLoader(posts.GetMany),
		comments: newLoader(comments.GetMany),
		roots: newLoader(func(ctx context.Context, keys []pageKey) ([]dto.CommentConnection, error) {
			inputs := make([]dto.RootPageInput, len(keys))
			for i, key := range keys {
				inputs[i] = dto.RootPageInput{PostID: key.id, PageInput: key.input()}
			}
			return comments.ListRootsBatch(ctx, inputs)
		}),
		replies: newLoader(func(ctx context.Context, keys []pageKey) ([]dto.CommentConnection, error) {
			inputs := make([]dto.ReplyPageInput, len(keys))
			for i, key := range keys {
				inputs[i] = dto.ReplyPageInput{ParentID: key.id, PageInput: key.input()}
			}
			return comments.ListRepliesBatch(ctx, inputs)
		}),
	}
}

func newLoader[K comparable, V any](fetch func(context.Context, []K) ([]V, error)) *dataloadgen.Loader[K, V] {
	return dataloadgen.NewLoader(func(ctx context.Context, keys []K) (values []V, errs []error) {
		// Fetch исполняется в горутине библиотеки вне recover gqlgen.
		defer func() {
			if recovered := recover(); recovered != nil {
				values, errs = nil, []error{fmt.Errorf("%w: batch panic: %v", domain.ErrStorage, recovered)}
			}
		}()
		if err := ctx.Err(); err != nil {
			return nil, []error{err}
		}
		values, err := fetch(ctx, keys)
		if err != nil {
			return nil, []error{err}
		}
		if len(values) != len(keys) {
			return nil, []error{fmt.Errorf("%w: batch returned %d values for %d keys", domain.ErrStorage, len(values), len(keys))}
		}
		return values, nil
	}, dataloadgen.WithBatchCapacity(page.MaxBatchSize), dataloadgen.WithWait(5*time.Millisecond))
}

// Мутации выполняются последовательно. Полная очистка простая и сохраняет актуальность
// страниц и ID при нескольких записях с вложенными чтениями в одной операции.
func (l *Loaders) Clear() {
	l.posts.ClearAll()
	l.comments.ClearAll()
	l.roots.ClearAll()
	l.replies.ClearAll()
}
