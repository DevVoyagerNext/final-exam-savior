package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"final-exam-savior/backend/internal/app"
	"final-exam-savior/backend/internal/config"
	httpserver "final-exam-savior/backend/internal/transport/http"
)

func main() {
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Errorf("load config: %w", err))
	}

	application, err := app.New(rootCtx, cfg)
	if err != nil {
		panic(fmt.Errorf("bootstrap app: %w", err))
	}
	defer func() {
		_ = application.Close()
	}()

	application.StartWorkers(rootCtx)

	server := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      httpserver.New(application).Router(),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		<-rootCtx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(fmt.Errorf("http server exit: %w", err))
	}
}
