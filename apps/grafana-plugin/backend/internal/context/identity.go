// Package context maps trusted Grafana plugin context into the shared request context.
package context

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	requestcontext "mini-torchbearing.local/packages/request-context-go"
)

func FromResourceRequest(request *backend.CallResourceRequest) (requestcontext.Context, bool) {
	if request == nil || request.PluginContext.User == nil || strings.TrimSpace(request.PluginContext.User.Login) == "" {
		return requestcontext.Context{}, false
	}
	user := request.PluginContext.User
	role := strings.TrimSpace(user.Role)
	if role == "" {
		role = "Viewer"
	}
	requestID, traceID := request.GetHTTPHeader("X-Request-ID"), request.GetHTTPHeader("X-Trace-ID")
	if requestID == "" {
		requestID = opaqueID()
	}
	if traceID == "" {
		traceID = opaqueID()
	}
	permissions := []string{"datasources:query", "incidents:read"}
	if role == "Admin" {
		permissions = append(permissions, "incidents:approve")
	}
	return requestcontext.Context{TenantID: "org:" + strconv.FormatInt(request.PluginContext.OrgID, 10), OrgID: strconv.FormatInt(request.PluginContext.OrgID, 10), UserID: user.Login, Roles: []string{role}, Permissions: permissions, RequestID: requestID, TraceID: traceID}, true
}

func opaqueID() string {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return "plugin-request"
	}
	return hex.EncodeToString(bytes)
}
