package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	generated "mini-torchbearing.local/services/ai-core/internal/adapters/inbound/http/generated"
	"mini-torchbearing.local/services/ai-core/internal/application/incidents"
	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	domainincident "mini-torchbearing.local/services/ai-core/internal/domain/incident"
)

const maxAlertWebhookBytes = 64 << 10

type AlertIngestor interface {
	Ingest(context.Context, incidents.Alert) (incidents.Result, error)
}

type AlertIngressConfig struct {
	SourceID     string
	OrgID        int
	HMACSecret   string
	MaxClockSkew time.Duration
	CurrentTime  func() time.Time
}

func (a *API) IngestGrafanaAlert(w http.ResponseWriter, r *http.Request, params generated.IngestGrafanaAlertParams) {
	if a == nil || a.Incidents == nil || errInvalidAlertConfig(a.AlertIngress) != nil {
		writeError(w, params.XRequestID, common.NewError(common.AdapterNotConfigured, "Grafana alert ingress is not configured", false))
		return
	}
	body, err := readBoundedAlertBody(r.Body)
	if err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	if err := verifyAlertRequest(a.AlertIngress, params, body); err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	var webhook generated.GrafanaAlertWebhookSchema
	if err := decodeStrictAlertJSON(body, &webhook); err != nil {
		writeError(w, params.XRequestID, err)
		return
	}
	if err := validateAlertWebhook(a.AlertIngress, webhook, body); err != nil {
		writeError(w, params.XRequestID, err)
		return
	}

	accepted, duplicate := 0, 0
	taskIDs := make([]string, 0, len(webhook.Alerts))
	seenTasks := make(map[string]bool, len(webhook.Alerts))
	for _, alert := range webhook.Alerts {
		result, err := a.Incidents.Ingest(r.Context(), incidents.Alert{
			SourceID: string(params.XMTBAlertSource), AlertName: alert.Labels.Alertname,
			Fingerprint: alert.Fingerprint, ServiceRef: alert.Labels.ServiceRef,
			Status: alertStatus(alert.Status), Labels: persistentAlertLabels(alert.Labels),
			StartsAt: alert.StartsAt, RequestID: params.XRequestID, TraceID: params.XTraceID,
		})
		if err != nil {
			writeError(w, params.XRequestID, err)
			return
		}
		if result.Accepted {
			accepted++
		}
		if result.Duplicate {
			duplicate++
		}
		if result.TaskID != "" && !seenTasks[result.TaskID] {
			seenTasks[result.TaskID] = true
			taskIDs = append(taskIDs, result.TaskID)
		}
	}
	writeJSON(w, http.StatusAccepted, generated.AlertIngressResponseSchema{Accepted: accepted, Duplicate: duplicate, TaskIds: taskIDs})
}

func readBoundedAlertBody(body io.Reader) ([]byte, error) {
	if body == nil {
		return nil, common.NewError(common.InvalidArgument, "Grafana alert body is required", false)
	}
	value, err := io.ReadAll(io.LimitReader(body, maxAlertWebhookBytes+1))
	if err != nil {
		return nil, common.NewError(common.InvalidArgument, "Grafana alert body could not be read", false)
	}
	if len(value) == 0 || len(value) > maxAlertWebhookBytes {
		return nil, common.NewError(common.InvalidArgument, "Grafana alert body exceeds the bounded size", false)
	}
	return value, nil
}

func verifyAlertRequest(config AlertIngressConfig, params generated.IngestGrafanaAlertParams, body []byte) error {
	unauthenticated := func() error {
		return common.NewError(common.Unauthenticated, "Grafana alert authentication failed", false)
	}
	if string(params.XMTBAlertSource) != config.SourceID {
		return unauthenticated()
	}
	now := config.CurrentTime().UTC()
	timestamp := time.Unix(int64(params.XMTBAlertTimestamp), 0).UTC()
	if timestamp.Before(now.Add(-config.MaxClockSkew)) || timestamp.After(now.Add(config.MaxClockSkew)) {
		return unauthenticated()
	}
	signatureText := string(params.XGrafanaAlertingSignature)
	if len(signatureText) != sha256.Size*2 || strings.ToLower(signatureText) != signatureText {
		return unauthenticated()
	}
	provided, err := hex.DecodeString(signatureText)
	if err != nil {
		return unauthenticated()
	}
	mac := hmac.New(sha256.New, []byte(config.HMACSecret))
	_, _ = mac.Write([]byte(strconv.FormatInt(int64(params.XMTBAlertTimestamp), 10)))
	_, _ = mac.Write([]byte(":"))
	_, _ = mac.Write(body)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return unauthenticated()
	}
	return nil
}

func decodeStrictAlertJSON(body []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return common.NewError(common.InvalidArgument, "invalid Grafana alert JSON", false)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return common.NewError(common.InvalidArgument, "Grafana alert body must contain one JSON value", false)
	}
	return nil
}

func validateAlertWebhook(config AlertIngressConfig, webhook generated.GrafanaAlertWebhookSchema, raw []byte) error {
	invalid := func() error {
		return common.NewError(common.SchemaValidationFailed, "Grafana alert payload violates the bounded contract", false)
	}
	if strings.TrimSpace(webhook.Receiver) == "" || len(webhook.Receiver) > 200 || !webhook.Status.Valid() || len(webhook.Alerts) < 1 || len(webhook.Alerts) > 10 {
		return invalid()
	}
	if webhook.OrgId != nil && *webhook.OrgId != config.OrgID {
		return invalid()
	}
	if !validOptionalString(webhook.ExternalURL, 500) || !validOptionalString(webhook.AppVersion, 80) || !validOptionalString(webhook.Version, 40) || !validOptionalString(webhook.GroupKey, 500) || !validOptionalString(webhook.Title, 500) || !validOptionalString(webhook.State, 80) || !validOptionalString(webhook.Message, 2000) || (webhook.TruncatedAlerts != nil && *webhook.TruncatedAlerts < 0) {
		return invalid()
	}
	for _, labels := range []*generated.Labels{webhook.GroupLabels, webhook.CommonLabels, webhook.CommonAnnotations} {
		if labels != nil && !validTransportLabels(*labels) {
			return invalid()
		}
	}
	if !requiredAlertFieldsPresent(raw, len(webhook.Alerts)) {
		return invalid()
	}
	for _, alert := range webhook.Alerts {
		if !alert.Status.Valid() || string(alert.Status) != string(webhook.Status) || strings.TrimSpace(alert.Labels.Alertname) == "" || strings.TrimSpace(alert.Labels.ServiceRef) == "" || len(alert.Labels.Alertname) > 200 || len(alert.Labels.ServiceRef) > 200 || strings.TrimSpace(alert.Fingerprint) == "" || len(alert.Fingerprint) > 200 || alert.StartsAt.IsZero() {
			return invalid()
		}
		if len(alert.Labels.AdditionalProperties)+2 > 24 || !validTransportLabels(alert.Annotations) || (alert.Values != nil && len(*alert.Values) > 24) {
			return invalid()
		}
		for _, value := range alert.Labels.AdditionalProperties {
			if len(value) > 500 {
				return invalid()
			}
		}
		for _, value := range []*string{alert.GeneratorURL, alert.SilenceURL, alert.DashboardURL, alert.PanelURL, alert.ImageURL} {
			if value != nil && len(*value) > 1000 {
				return invalid()
			}
		}
		if alert.ValueString != nil && len(*alert.ValueString) > 2000 {
			return invalid()
		}
	}
	return nil
}

func validOptionalString(value *string, maximum int) bool {
	return value == nil || len(*value) <= maximum
}

func validTransportLabels(value generated.Labels) bool {
	if len(value) > 24 {
		return false
	}
	for _, item := range value {
		if len(item) > 500 {
			return false
		}
	}
	return true
}

func requiredAlertFieldsPresent(raw []byte, expected int) bool {
	var root struct {
		Receiver json.RawMessage   `json:"receiver"`
		Status   json.RawMessage   `json:"status"`
		Alerts   []json.RawMessage `json:"alerts"`
	}
	if json.Unmarshal(raw, &root) != nil || root.Receiver == nil || root.Status == nil || len(root.Alerts) != expected {
		return false
	}
	for _, encoded := range root.Alerts {
		var fields map[string]json.RawMessage
		if json.Unmarshal(encoded, &fields) != nil {
			return false
		}
		for _, name := range []string{"status", "labels", "annotations", "startsAt", "endsAt", "fingerprint"} {
			if fields[name] == nil {
				return false
			}
		}
	}
	return true
}

func persistentAlertLabels(value generated.Alert_Labels) map[string]string {
	result := map[string]string{"alertname": value.Alertname, "service_ref": value.ServiceRef}
	for key, item := range value.AdditionalProperties {
		if len(item) <= 200 {
			result[key] = item
		}
	}
	return result
}

func alertStatus(value generated.AlertStatus) domainincident.AlertStatus {
	if string(value) == string(domainincident.AlertResolved) {
		return domainincident.AlertResolved
	}
	return domainincident.AlertFiring
}

func errInvalidAlertConfig(config AlertIngressConfig) error {
	if strings.TrimSpace(config.SourceID) == "" || config.OrgID < 1 || len(config.HMACSecret) < 16 || config.MaxClockSkew <= 0 || config.CurrentTime == nil {
		return common.NewError(common.AdapterNotConfigured, "Grafana alert ingress is not configured", false)
	}
	return nil
}
