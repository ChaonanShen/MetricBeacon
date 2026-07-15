package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	orderdemo "mini-torchbearing.local/packages/generated-clients/go/orderdemo"
)

func main() {
	endpoint := envOr("ORDER_DEMO_BUSINESS_ENDPOINT", "http://order-service:8090")
	rate, err := strconv.Atoi(envOr("ORDER_DEMO_LOAD_PER_SECOND", "2"))
	if err != nil || rate < 1 || rate > 20 {
		log.Fatal("ORDER_DEMO_LOAD_PER_SECOND must be between 1 and 20")
	}
	client, err := orderdemo.NewClientWithResponses(endpoint)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	ticker := time.NewTicker(time.Second / time.Duration(rate))
	defer ticker.Stop()
	sequence := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sequence++
			key := fmt.Sprintf("loadgen-%d-%d", time.Now().Unix(), sequence)
			requestCtx, requestCancel := context.WithTimeout(ctx, 2*time.Second)
			response, callErr := client.CreateOrderWithResponse(requestCtx, &orderdemo.CreateOrderParams{IdempotencyKey: key}, orderdemo.CreateOrderRequest{Sku: "DEMO", Quantity: 1})
			requestCancel()
			if callErr != nil || response.StatusCode() != 202 {
				log.Printf("order submission failed status=%d err=%v", responseStatus(response), callErr)
			}
		}
	}
}

func responseStatus(response *orderdemo.CreateOrderResponse) int {
	if response == nil {
		return 0
	}
	return response.StatusCode()
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
