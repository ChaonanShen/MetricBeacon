package bootstrap

import (
	"mini-torchbearing.local/apps/grafana-plugin/backend/internal/aicore"
	"mini-torchbearing.local/apps/grafana-plugin/backend/internal/config"
	"mini-torchbearing.local/apps/grafana-plugin/backend/internal/handlers"
)

func Wire(config config.Config) (*handlers.ResourceHandler, error) {
	client, err := aicore.New(config.AICoreEndpoint, config.Timeout)
	if err != nil {
		return nil, err
	}
	return &handlers.ResourceHandler{Client: client, MaxResponse: config.MaxResponse}, nil
}
