package subscription

import "errors"

var (
	ErrLimitReached = errors.New("достигнут лимит подписок")
	ErrClosed       = errors.New("сервис подписок остановлен")
)
