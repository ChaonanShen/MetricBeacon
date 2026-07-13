// Package requestcontext contains shared, transport-neutral request identity values.
package requestcontext

// Context is built by the Plugin Backend and copied through every AI Core and MCP call.
// It intentionally carries no credential or mutable transport state.
type Context struct {
	TenantID    string
	OrgID       string
	UserID      string
	Roles       []string
	Permissions []string
	RequestID   string
	TraceID     string
}

func (c Context) HasPermission(permission string) bool {
	for _, candidate := range c.Permissions {
		if candidate == permission {
			return true
		}
	}
	return false
}
