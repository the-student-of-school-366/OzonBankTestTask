package loader

import "context"

type contextKey struct{}

func WithContext(ctx context.Context, loaders *Loaders) context.Context {
	return context.WithValue(ctx, contextKey{}, loaders)
}

// FromContext вызывается резолверами после установки загрузчиков middleware операции.
func FromContext(ctx context.Context) *Loaders {
	return ctx.Value(contextKey{}).(*Loaders)
}
