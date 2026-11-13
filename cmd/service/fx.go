package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"OzonBankTestTask/internal/api"
	"OzonBankTestTask/internal/api/handler"
	"OzonBankTestTask/internal/api/sandbox"
	"OzonBankTestTask/internal/broker"
	"OzonBankTestTask/internal/infrastructure/pubsub"
	"OzonBankTestTask/internal/repositories/memory"
	"OzonBankTestTask/internal/repositories/postgres"
	commentrepo "OzonBankTestTask/internal/repositories/postgres/comment"
	postrepo "OzonBankTestTask/internal/repositories/postgres/post"
	commentusecase "OzonBankTestTask/internal/usecase/comment"
	postusecase "OzonBankTestTask/internal/usecase/post"
	"OzonBankTestTask/internal/usecase/subscription"

	"go.uber.org/fx"
)

// cleanup закрывает зависимости в обратном порядке регистрации.
// Вызов безопасен повторно: так останавливаются и неудачная сборка, и обычный выход.
type cleanup struct {
	closers []func()
}

func (c *cleanup) Add(close func()) {
	if close != nil {
		c.closers = append(c.closers, close)
	}
}

func (c *cleanup) Close() {
	closers := c.closers
	c.closers = nil
	for i := len(closers) - 1; i >= 0; i-- {
		closers[i]()
	}
}

func newApplication(ctx context.Context, cfg config) (*application, error) {
	resources := &cleanup{}
	var app *application
	graph := fx.New(
		fx.NopLogger,
		fx.Supply(cfg, resources),
		fx.Provide(func() context.Context { return ctx }),
		fx.Provide(
			newStorage,
			newHub,
			newPostUseCase,
			newCommentUseCase,
			newSubscriptionUseCase,
			newGraphQL,
			newServeMux,
			newHTTPServer,
			newApp,
		),
		fx.Provide(
			func(hub *pubsub.Hub) commentusecase.Publisher { return hub },
			func(hub *pubsub.Hub) broker.Broker { return hub },
			func(posts *postusecase.UseCase) subscription.PostReader { return posts },
			func(posts *postusecase.UseCase) api.PostUseCase { return posts },
			func(comments *commentusecase.UseCase) api.CommentUseCase { return comments },
			func(events *subscription.UseCase) api.SubscriptionUseCase { return events },
			func() api.Config { return api.Config{} },
		),
		fx.Populate(&app),
	)
	if err := graph.Err(); err != nil {
		resources.Close()
		return nil, err
	}
	return app, nil
}

func newStorage(ctx context.Context, cfg config, resources *cleanup) (postusecase.Repository, commentusecase.Repository, error) {
	switch cfg.storage {
	case "memory":
		store := memory.New()
		slog.Info("storage ready", "storage", "memory")
		return store, store, nil
	case "postgres":
		settings := postgres.Config{DSN: cfg.databaseURL}
		startup, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if err := postgres.Migrate(startup, settings); err != nil {
			return nil, nil, fmt.Errorf("apply migrations: %w", err)
		}
		slog.Info("migrations applied")
		pool, err := postgres.Open(startup, settings)
		if err != nil {
			return nil, nil, fmt.Errorf("open postgres: %w", err)
		}
		resources.Add(pool.Close)
		slog.Info("storage ready", "storage", "postgres")
		return postrepo.New(pool), commentrepo.New(pool), nil
	default:
		return nil, nil, fmt.Errorf("unknown storage %q", cfg.storage)
	}
}

func newHub(resources *cleanup) (*pubsub.Hub, error) {
	hub, err := pubsub.New(pubsub.Config{})
	if err != nil {
		return nil, fmt.Errorf("create event hub: %w", err)
	}
	resources.Add(hub.Close)
	return hub, nil
}

func newPostUseCase(repository postusecase.Repository) *postusecase.UseCase {
	return postusecase.New(repository)
}

func newCommentUseCase(repository commentusecase.Repository, publisher commentusecase.Publisher) *commentusecase.UseCase {
	return commentusecase.New(repository, publisher)
}

func newSubscriptionUseCase(posts subscription.PostReader, events broker.Broker) *subscription.UseCase {
	return subscription.New(posts, events)
}

func newGraphQL(posts api.PostUseCase, comments api.CommentUseCase, events api.SubscriptionUseCase, settings api.Config, resources *cleanup) (*handler.Handler, error) {
	graph, err := handler.NewHandler(posts, comments, events, settings)
	if err != nil {
		return nil, fmt.Errorf("create GraphQL handler: %w", err)
	}
	resources.Add(graph.Close)
	return graph, nil
}

func newServeMux(cfg config, graph *handler.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/graphql", graph)
	if cfg.sandboxEnabled {
		mux.Handle("/sandbox", sandbox.NewHandler())
		slog.Info("apollo sandbox enabled", "path", "/sandbox")
	}
	return mux
}

func newHTTPServer(cfg config, mux *http.ServeMux) *http.Server {
	return &http.Server{
		Addr:              cfg.address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func newApp(server *http.Server, graph *handler.Handler, hub *pubsub.Hub, resources *cleanup) *application {
	return &application{server: server, graph: graph, hub: hub, resources: resources}
}
