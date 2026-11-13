package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"OzonBankTestTask/internal/api"
	"OzonBankTestTask/internal/api/handler"
	"OzonBankTestTask/internal/infrastructure/pubsub"
	"OzonBankTestTask/internal/repositories/memory"
	"OzonBankTestTask/internal/repositories/postgres"
	commentrepo "OzonBankTestTask/internal/repositories/postgres/comment"
	postrepo "OzonBankTestTask/internal/repositories/postgres/post"
	commentusecase "OzonBankTestTask/internal/usecase/comment"
	postusecase "OzonBankTestTask/internal/usecase/post"
	"OzonBankTestTask/internal/usecase/subscription"
)

type application struct {
	server       *http.Server
	graph        *handler.Handler
	hub          *pubsub.Hub
	closeStorage func()
}

func newApplication(ctx context.Context, c config) (*application, error) {
	var posts postusecase.Repository
	var comments commentusecase.Repository
	closeStorage := func() {}
	switch c.storage {
	case "memory":
		store := memory.New()
		posts, comments = store, store
	case "postgres":
		settings := postgres.Config{DSN: c.databaseURL}
		startup, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if err := postgres.Migrate(startup, settings); err != nil {
			return nil, fmt.Errorf("apply migrations: %w", err)
		}
		pool, err := postgres.Open(startup, settings)
		if err != nil {
			return nil, fmt.Errorf("open postgres: %w", err)
		}
		posts, comments = postrepo.New(pool), commentrepo.New(pool)
		closeStorage = pool.Close
	default:
		return nil, fmt.Errorf("unknown storage %q", c.storage)
	}
	hub, err := pubsub.New(pubsub.Config{})
	if err != nil {
		closeStorage()
		return nil, fmt.Errorf("create event hub: %w", err)
	}
	postService := postusecase.New(posts)
	commentService := commentusecase.New(comments, hub)
	graph, err := handler.NewHandler(postService, commentService, subscription.New(postService, hub), api.Config{})
	if err != nil {
		hub.Close()
		closeStorage()
		return nil, fmt.Errorf("create GraphQL handler: %w", err)
	}
	mux := http.NewServeMux()
	mux.Handle("/graphql", graph)
	return &application{
		server: &http.Server{Addr: c.address, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second},
		graph:  graph, hub: hub, closeStorage: closeStorage,
	}, nil
}

func (a *application) close() {
	a.graph.Close()
	a.hub.Close()
	a.closeStorage()
}

func (a *application) serve(ctx context.Context, listener net.Listener, timeout time.Duration) error {
	defer a.close()
	finished := make(chan error, 1)
	go func() { finished <- a.server.Serve(listener) }()
	select {
	case err := <-finished:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		// Shutdown не закрывает WebSocket после upgrade.
		a.graph.Close()
		shutdown, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		err := a.server.Shutdown(shutdown)
		if err != nil {
			_ = a.server.Close()
			<-finished
			return fmt.Errorf("shutdown HTTP: %w", err)
		}
		if err = <-finished; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	}
}

func run(ctx context.Context) error {
	c, err := loadConfig()
	if err != nil {
		return err
	}
	app, err := newApplication(ctx, c)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", c.address)
	if err != nil {
		app.close()
		return fmt.Errorf("listen: %w", err)
	}
	slog.Info("service started", "address", listener.Addr().String(), "storage", c.storage, "graphql", "/graphql")
	return app.serve(ctx, listener, c.shutdownTimeout)
}
