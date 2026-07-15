package common

import "fmt"

type ErrorCode string

const (
	InvalidArgument          ErrorCode = "invalid_argument"
	Unauthenticated          ErrorCode = "unauthenticated"
	PermissionDenied         ErrorCode = "permission_denied"
	ResourceNotFound         ErrorCode = "resource_not_found"
	ResourceConflict         ErrorCode = "resource_conflict"
	InvalidStateTransition   ErrorCode = "invalid_state_transition"
	AdapterNotConfigured     ErrorCode = "adapter_not_configured"
	DependencyUnavailable    ErrorCode = "dependency_unavailable"
	ToolNotSupported         ErrorCode = "tool_not_supported"
	ToolTimeout              ErrorCode = "tool_timeout"
	SchemaValidationFailed   ErrorCode = "schema_validation_failed"
	IdempotencyConflict      ErrorCode = "idempotency_conflict"
	ExecutionInterrupted     ErrorCode = "execution_interrupted"
	TargetPreconditionFailed ErrorCode = "target_precondition_failed"
	ApprovalRequired         ErrorCode = "approval_required"
	ApprovalExpired          ErrorCode = "approval_expired"
	InternalError            ErrorCode = "internal_error"
	NotImplemented           ErrorCode = "not_implemented"
)

// DomainError is the only error shape that adapters may expose above their boundary.
type DomainError struct {
	Code      ErrorCode
	Message   string
	Retryable bool
	Details   map[string]string
}

func (e *DomainError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

func NewError(code ErrorCode, message string, retryable bool) *DomainError {
	return &DomainError{Code: code, Message: message, Retryable: retryable}
}
