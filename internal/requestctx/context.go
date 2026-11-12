package requestctx

import (
	"context"
	"errors"
)

type contextKey uint8

const (
	actorKey contextKey = iota
	requestIDKey
)

var ErrUnauthenticated = errors.New("требуется идентификатор пользователя")

func WithActor(ctx context.Context, actorID string) context.Context {
	return context.WithValue(ctx, actorKey, actorID)
}

// HasActor различает отсутствие actor и явно заданное пустое значение.
func HasActor(ctx context.Context) bool {
	return ctx.Value(actorKey) != nil
}

func Actor(ctx context.Context) (string, error) {
	actor, _ := ctx.Value(actorKey).(string)
	if actor == "" {
		return "", ErrUnauthenticated
	}
	return actor, nil
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}
