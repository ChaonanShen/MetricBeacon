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
