// Package runtime contains transport-neutral MCP boundary helpers.
package runtime

import "fmt"

type ErrorCode string

const (
	InvalidArgument        ErrorCode = "invalid_argument"
	Unauthenticated        ErrorCode = "unauthenticated"
	PermissionDenied       ErrorCode = "permission_denied"
	ResourceNotFound       ErrorCode = "resource_not_found"
	AdapterNotConfigured   ErrorCode = "adapter_not_configured"
	DependencyUnavailable  ErrorCode = "dependency_unavailable"
	ToolNotSupported       ErrorCode = "tool_not_supported"
	ToolTimeout            ErrorCode = "tool_timeout"
	SchemaValidationFailed ErrorCode = "schema_validation_failed"
	NotImplemented         ErrorCode = "not_implemented"
	InternalError          ErrorCode = "internal_error"
)

// ToolError is the classified error shape exchanged across the namespace and
// transport boundary. It never includes a driver, filesystem, or SDK error.
type ToolError struct {
	Code      ErrorCode         `json:"code"`
	Message   string            `json:"message"`
	Retryable bool              `json:"retryable"`
	Details   map[string]string `json:"details,omitempty"`
}

func (e *ToolError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

func NewError(code ErrorCode, message string, retryable bool) *ToolError {
	return &ToolError{Code: code, Message: message, Retryable: retryable}
}
