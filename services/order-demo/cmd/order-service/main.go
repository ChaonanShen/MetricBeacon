package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	httpadapter "mini-torchbearing.local/services/order-demo/internal/adapters/inbound/http"
	prometheusadapter "mini-torchbearing.local/services/order-demo/internal/adapters/outbound/prometheus"
	"mini-torchbearing.local/services/order-demo/internal/application"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	readToken := os.Getenv("ORDER_DEMO_READ_TOKEN")
	remediationToken := os.Getenv("ORDER_DEMO_REMEDIATION_TOKEN")
	if readToken == "" || remediationToken == "" || readToken == remediationToken {
		return errors.New("ORDER_DEMO_READ_TOKEN and ORDER_DEMO_REMEDIATION_TOKEN must be non-empty and different")
	}
	faultSocket := envOr("ORDER_DEMO_FAULT_SOCKET", "/run/order-demo/control.sock")
	if err := os.MkdirAll(filepath.Dir(faultSocket), 0o700); err != nil {
		return fmt.Errorf("create fault socket directory: %w", err)
	}
	if err := os.Remove(faultSocket); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove old fault socket: %w", err)
	}

	metrics := prometheusadapter.NewRecorder()
	engine := application.NewEngine(application.Options{Metrics: metrics})
	defer engine.Close()
	metrics.BindEngine(engine)
	api := httpadapter.NewAPI(engine)

	unixListener, err := net.Listen("unix", faultSocket)
	if err != nil {
		return fmt.Errorf("listen on fault socket: %w", err)
	}
	defer func() {
		_ = unixListener.Close()
		_ = os.Remove(faultSocket)
	}()
	if err := os.Chmod(faultSocket, 0o600); err != nil {
		return fmt.Errorf("protect fault socket: %w", err)
	}

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", metrics.Handler())
	metricsMux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	servers := []struct {
		name     string
		server   *http.Server
		listener net.Listener
	}{
		{name: "business", server: &http.Server{Addr: envOr("ORDER_DEMO_BUSINESS_ADDRESS", ":8090"), Handler: api.BusinessHandler(), ReadHeaderTimeout: 2 * time.Second}},
		{name: "operational", server: &http.Server{Addr: envOr("ORDER_DEMO_OPERATIONAL_ADDRESS", ":8091"), Handler: api.OperationalHandler(readToken, remediationToken), ReadHeaderTimeout: 2 * time.Second}},
		{name: "metrics", server: &http.Server{Addr: envOr("ORDER_DEMO_METRICS_ADDRESS", ":9102"), Handler: metricsMux, ReadHeaderTimeout: 2 * time.Second}},
		{name: "fault-socket", server: &http.Server{Handler: api.FaultHandler(), ReadHeaderTimeout: 2 * time.Second}, listener: unixListener},
	}

	errCh := make(chan error, len(servers))
	for i := range servers {
		entry := &servers[i]
		go func() {
			var serveErr error
			if entry.listener != nil {
				serveErr = entry.server.Serve(entry.listener)
			} else {
				serveErr = entry.server.ListenAndServe()
			}
			if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				errCh <- fmt.Errorf("%s server: %w", entry.name, serveErr)
			}
		}()
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		return err
	case <-signals:
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for i := range servers {
		_ = servers[i].server.Shutdown(ctx)
	}
	return nil
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
