package errors

import "encoding/json"

func Envelope(code, message, requestID string, retryable bool) []byte {
	value, _ := json.Marshal(map[string]any{"error": map[string]any{"code": code, "message": message, "retryable": retryable, "requestId": requestID}})
	return value
}
