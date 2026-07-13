package main

import (
	"log"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"mini-torchbearing.local/apps/grafana-plugin/backend/internal/bootstrap"
	"mini-torchbearing.local/apps/grafana-plugin/backend/internal/config"
)

func main() {
	handler, err := bootstrap.Wire(config.Load())
	if err != nil {
		log.Fatal(err)
	}
	if err := backend.Serve(backend.ServeOpts{CallResourceHandler: handler}); err != nil {
		log.Fatal(err)
	}
}
