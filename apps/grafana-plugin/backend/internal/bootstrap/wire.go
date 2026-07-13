package bootstrap

import (
	"mini-torchbearing.local/apps/grafana-plugin/backend/internal/aicore"
	"mini-torchbearing.local/apps/grafana-plugin/backend/internal/config"
	"mini-torchbearing.local/apps/grafana-plugin/backend/internal/handlers"
	generated "mini-torchbearing.local/packages/generated-clients/go"
)

func Wire(config config.Config) (*handlers.ResourceHandler, error) {
	return &handlers.ResourceHandler{
		NewClient: func(endpoint string) (generated.ClientInterface, error) {
			return aicore.New(endpoint, config.Timeout)
		},
		MaxResponse: config.MaxResponse,
	}, nil
}
