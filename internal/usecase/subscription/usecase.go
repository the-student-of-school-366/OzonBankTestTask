package subscription

import (
	"context"

	"OzonBankTestTask/internal/broker"
	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/dto"
)

type UseCase struct {
	posts  PostReader
	broker broker.Broker
}

func New(posts PostReader, events broker.Broker) *UseCase {
	return &UseCase{posts: posts, broker: events}
}

// CommentAdded подписывает на новые корни и ответы существующего поста.
// История не воспроизводится; разрешение комментирования не ограничивает чтение.
func (u *UseCase) CommentAdded(ctx context.Context, postID string) (<-chan dto.Comment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	id, err := dto.ParseID(postID)
	if err != nil {
		return nil, err
	}
	posts, err := u.posts.GetMany(ctx, []string{id.String()})
	if err != nil {
		return nil, err
	}
	if len(posts) != 1 {
		return nil, domain.ErrStorage
	}
	if posts[0] == nil {
		return nil, domain.ErrPostNotFound
	}
	stream, err := u.broker.Subscribe(ctx, id)
	if err != nil {
		return nil, err
	}
	output := make(chan dto.Comment)
	go func() {
		defer close(output)
		defer stream.Cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stream.Done:
				return
			case value, ok := <-stream.Comments:
				if !ok {
					return
				}
				select {
				case output <- dto.CommentFromDomain(value):
				case <-ctx.Done():
					return
				case <-stream.Done:
					return
				}
			}
		}
	}()
	return output, nil
}
