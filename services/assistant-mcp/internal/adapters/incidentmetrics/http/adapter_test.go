package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"testing"
	"time"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
)

func TestHTTPAdapterUsesOnlyRegisteredInstantQueries(t *testing.T) {
	now := time.Date(2026, 7, 16, 14, 0, 0, 0, time.UTC)
	seen := make([]string, 0, 4)
	server := httptest.NewServer(stdhttp.HandlerFunc(func(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
		if request.Method != stdhttp.MethodGet || request.URL.Path != "/api/v1/query" || request.URL.Query().Get("time") == "" {
			t.Fatalf("request: %s %s", request.Method, request.URL.String())
		}
		expression := request.URL.Query().Get("query")
		seen = append(seen, expression)
		value := "10"
		if expression == registeredQueries["depth"] || expression == registeredQueries["oldest"] {
			value = "0"
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"status": "success", "data": map[string]any{"resultType": "vector", "result": []any{map[string]any{"metric": map[string]string{}, "value": []any{now.Unix(), value}}}}})
	}))
	t.Cleanup(server.Close)
	parsed, _ := url.Parse(server.URL)
	adapter, err := newWithClient(parsed, server.Client(), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.GetRecovery(context.Background(), requestcontext.Context{})
	if err != nil || result.WindowSeconds != 30 || result.AcceptedDelta != 10 || result.CompletedDelta != 10 || result.QueueDepth != 0 || result.OldestAgeSeconds != 0 || !result.ObservedAt.Equal(now) {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	want := make([]string, 0, len(registeredQueries))
	for _, expression := range registeredQueries {
		want = append(want, expression)
	}
	sort.Strings(want)
	sort.Strings(seen)
	if len(seen) != 4 {
		t.Fatalf("queries=%#v", seen)
	}
	for index := range want {
		if seen[index] != want[index] {
			t.Fatalf("queries=%#v", seen)
		}
	}
}

func TestHTTPAdapterRejectsMalformedAndUnboundedResponses(t *testing.T) {
	for name, handler := range map[string]stdhttp.HandlerFunc{
		"empty": func(writer stdhttp.ResponseWriter, _ *stdhttp.Request) {
			_, _ = writer.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
		},
		"negative": func(writer stdhttp.ResponseWriter, _ *stdhttp.Request) {
			_, _ = writer.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"value":[1,"-1"]}]}}`))
		},
		"oversized": func(writer stdhttp.ResponseWriter, _ *stdhttp.Request) {
			_, _ = writer.Write(make([]byte, maxBodyBytes+1))
		},
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(handler)
			t.Cleanup(server.Close)
			adapter, _ := New(server.URL, time.Second)
			if _, err := adapter.GetRecovery(context.Background(), requestcontext.Context{}); err == nil {
				t.Fatal("invalid response was accepted")
			}
		})
	}
}
