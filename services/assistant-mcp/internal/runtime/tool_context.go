package runtime

import (
	"net/http"
	"strings"

	requestcontext "mini-torchbearing.local/packages/request-context-go"
)

const (
	HeaderTenantID    = "X-MTB-Tenant-ID"
	HeaderOrgID       = "X-MTB-Org-ID"
	HeaderUserID      = "X-MTB-User-ID"
	HeaderRoles       = "X-MTB-Roles"
	HeaderPermissions = "X-MTB-Permissions"
	HeaderRequestID   = "X-Request-ID"
	HeaderTraceID     = "X-Trace-ID"

	PermissionDatasourceQuery   = "datasources:query"
	PermissionIncidentDiagnose  = "incidents:diagnose"
	PermissionIncidentRemediate = "incidents:remediate"
)

// RequestContextFromHeaders parses only the internal headers constructed by
// Plugin Backend and forwarded by AI Core. It has no global mutable state.
func RequestContextFromHeaders(headers http.Header) (requestcontext.Context, error) {
	context := requestcontext.Context{
		TenantID:    strings.TrimSpace(headers.Get(HeaderTenantID)),
		OrgID:       strings.TrimSpace(headers.Get(HeaderOrgID)),
		UserID:      strings.TrimSpace(headers.Get(HeaderUserID)),
		Roles:       splitHeaderList(headers.Get(HeaderRoles)),
		Permissions: splitHeaderList(headers.Get(HeaderPermissions)),
		RequestID:   strings.TrimSpace(headers.Get(HeaderRequestID)),
		TraceID:     strings.TrimSpace(headers.Get(HeaderTraceID)),
	}
	if context.TenantID == "" || context.OrgID == "" || context.UserID == "" {
		return requestcontext.Context{}, NewError(Unauthenticated, "tenant, organization and user context are required", false)
	}
	return context, nil
}

func RequirePermission(context requestcontext.Context, permission string) error {
	if !context.HasPermission(permission) {
		return NewError(PermissionDenied, "required permission is missing", false)
	}
	return nil
}

func splitHeaderList(value string) []string {
	seen := make(map[string]struct{})
	items := make([]string, 0)
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		items = append(items, item)
	}
	return items
}
