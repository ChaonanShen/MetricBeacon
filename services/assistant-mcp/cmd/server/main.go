package main

import (
	"log"
	"net/http"

	"mini-torchbearing.local/services/assistant-mcp/internal/bootstrap"
)

func main() {
	config, err := bootstrap.LoadConfigFromEnvironment()
	if err != nil {
		log.Fatal(err)
	}
	runtime, err := bootstrap.Wire(config)
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Addr: config.ListenAddress, Handler: runtime.Handler}
	log.Printf("assistant-mcp listening on %s", config.ListenAddress)
	log.Fatal(server.ListenAndServe())
}
