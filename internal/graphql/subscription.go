package graphql

import (
	"context"

	"OzonBankTestTask/internal/dto"
)

func (r *Resolver) commentEvents(ctx context.Context, postID string) (<-chan *dto.Comment, error) {
	comments, err := r.subscriptions.CommentAdded(ctx, postID)
	if err != nil {
		return nil, err
	}
	events := make(chan *dto.Comment)
	go func() {
		defer close(events)
		for {
			select {
			case <-ctx.Done():
				return
			case comment, ok := <-comments:
				if !ok {
					return
				}
				select {
				case events <- &comment:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return events, nil
}
