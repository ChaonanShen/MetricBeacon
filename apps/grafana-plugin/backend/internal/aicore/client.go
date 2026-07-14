// Package aicore contains only the generated AI Core HTTP client wiring.
package aicore

import (
	"net/http"
	"time"

	generated "mini-torchbearing.local/packages/generated-clients/go"
)

func New(endpoint string, timeout time.Duration) (generated.ClientInterface, error) {
	return generated.NewClient(endpoint, generated.WithHTTPClient(&http.Client{Timeout: timeout}))
}

// NewStream deliberately leaves Client.Timeout unset. An SSE response may stay
// open while its Task is active; cancellation is controlled by the request
// context supplied by Grafana instead of an arbitrary client-wide deadline.
func NewStream(endpoint string) (generated.ClientInterface, error) {
	return generated.NewClient(endpoint, generated.WithHTTPClient(&http.Client{}))
}
