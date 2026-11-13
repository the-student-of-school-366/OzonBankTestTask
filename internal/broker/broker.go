// Package broker задаёт доставку событий подписки внутри процесса.
// Сценарий подписки и хаб зависят от этого контракта, но не друг от друга.
package broker

import (
	"context"

	"OzonBankTestTask/internal/domain/comment"

	"github.com/google/uuid"
)

//go:generate go tool mockery --case=snake --all --exclude=mocks

// Stream принадлежит реализации брокера. Cancel идемпотентен, Done закрывается при любой отписке.
type Stream struct {
	Comments <-chan comment.Comment
	Done     <-chan struct{}
	Cancel   func()
}

type Broker interface {
	Subscribe(context.Context, uuid.UUID) (Stream, error)
}

type Error string

func (e Error) Error() string { return string(e) }

// Code возвращает код ответа API. Неизвестный код остаётся внутренней ошибкой.
func (e Error) Code() string {
	switch e {
	case ErrLimitReached:
		return "RESOURCE_EXHAUSTED"
	case ErrClosed:
		return "UNAVAILABLE"
	default:
		return "INTERNAL"
	}
}

const (
	ErrLimitReached Error = "достигнут лимит подписок"
	ErrClosed       Error = "сервис подписок остановлен"
)
