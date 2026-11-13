package pubsub

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"OzonBankTestTask/internal/broker"
	"OzonBankTestTask/internal/domain/comment"

	"github.com/google/uuid"
)

type Config struct {
	BufferSize       int
	MaxSubscriptions int
}

// Hub доставляет события только внутри процесса, без истории и гарантий replay.
type Hub struct {
	mu     sync.Mutex
	config Config
	byPost map[uuid.UUID]map[*subscriber]struct{}
	count  int
	closed bool
}

type subscriber struct {
	postID   uuid.UUID
	comments chan comment.Comment
	done     chan struct{}
	stop     func() bool
}

func New(config Config) (*Hub, error) {
	if config.BufferSize == 0 {
		config.BufferSize = 64
	}
	if config.MaxSubscriptions == 0 {
		config.MaxSubscriptions = 1024
	}
	if config.BufferSize < 1 || config.MaxSubscriptions < 1 {
		return nil, fmt.Errorf("pubsub: invalid subscription limits")
	}
	return &Hub{config: config, byPost: make(map[uuid.UUID]map[*subscriber]struct{})}, nil
}

func (h *Hub) Subscribe(ctx context.Context, postID uuid.UUID) (broker.Stream, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return broker.Stream{}, err
	}
	if h.closed {
		slog.WarnContext(ctx, "subscription rejected", "reason", "hub stopped", "post_id", postID)
		return broker.Stream{}, broker.ErrClosed
	}
	if h.count >= h.config.MaxSubscriptions {
		slog.WarnContext(ctx, "subscription rejected", "reason", "limit", "post_id", postID)
		return broker.Stream{}, broker.ErrLimitReached
	}
	s := &subscriber{postID: postID, comments: make(chan comment.Comment, h.config.BufferSize), done: make(chan struct{})}
	if h.byPost[postID] == nil {
		h.byPost[postID] = make(map[*subscriber]struct{})
	}
	h.byPost[postID][s] = struct{}{}
	h.count++
	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.remove(s)
	}
	s.stop = context.AfterFunc(ctx, cancel)
	slog.InfoContext(ctx, "subscription started", "post_id", postID, "subscriptions", h.count)
	return broker.Stream{Comments: s.comments, Done: s.done, Cancel: cancel}, nil
}

var _ broker.Broker = (*Hub)(nil)

// Publish не ждёт читателей. Переполнение завершает только медленную подписку.
// Отправка и закрытие канала защищены одним mutex, копия ParentID своя для каждого читателя.
func (h *Hub) Publish(value comment.Comment) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.byPost[value.PostID] {
		select {
		case s.comments <- value.Clone():
		default:
			slog.Warn("subscriber dropped", "reason", "queue full", "post_id", value.PostID)
			h.remove(s)
		}
	}
}

func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.closed {
		slog.Info("subscription hub stopped", "subscriptions", h.count)
	}
	h.closed = true
	for _, subscribers := range h.byPost {
		for s := range subscribers {
			h.remove(s)
		}
	}
}

// remove вызывается только под h.mu; канал закрывает исключительно хаб.
func (h *Hub) remove(s *subscriber) {
	subscribers := h.byPost[s.postID]
	if _, exists := subscribers[s]; !exists {
		return
	}
	delete(subscribers, s)
	if len(subscribers) == 0 {
		delete(h.byPost, s.postID)
	}
	h.count--
	s.stop()
	close(s.done)
	close(s.comments)
}
