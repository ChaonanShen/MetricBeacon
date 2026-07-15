package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"
)

func main() {
	socket := envOr("ORDER_DEMO_FAULT_SOCKET", "/run/order-demo/control.sock")
	upstream, _ := url.Parse("http://order-demo-fault-socket")
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.Transport = &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "unix", socket)
	}}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.Handle("/faults/", proxy)
	server := &http.Server{Addr: envOr("ORDER_DEMO_FAULT_ADDRESS", ":8092"), Handler: mux, ReadHeaderTimeout: 2 * time.Second}
	log.Fatal(server.ListenAndServe())
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
