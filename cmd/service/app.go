package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"OzonBankTestTask/internal/api/handler"
	"OzonBankTestTask/internal/infrastructure/pubsub"
)

type application struct {
	server    *http.Server
	graph     *handler.Handler
	hub       *pubsub.Hub
	resources *cleanup
}

func (a *application) close() {
	if a.resources != nil {
		a.resources.Close()
	}
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
		slog.Info("shutdown started", "timeout", timeout.String())
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
		slog.Info("shutdown finished")
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
