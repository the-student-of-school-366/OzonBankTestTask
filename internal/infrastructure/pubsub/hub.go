package pubsub

import (
	"context"
	"fmt"
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
		return broker.Stream{}, broker.ErrClosed
	}
	if h.count >= h.config.MaxSubscriptions {
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
			h.remove(s)
		}
	}
}

func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
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
