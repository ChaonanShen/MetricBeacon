package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpapi "mini-torchbearing.local/services/ai-core/internal/adapters/inbound/http"
	"mini-torchbearing.local/services/ai-core/internal/bootstrap"
)

func main() {
	config, err := bootstrap.LoadConfigFromEnvironment()
	if err != nil {
		log.Fatal(err)
	}
	application, err := bootstrap.New(context.Background(), config)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = application.Close() }()
	server := &http.Server{Addr: config.ListenAddress, Handler: httpapi.NewHandler(&application.Handler), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
