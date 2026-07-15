package incident

import (
	"strings"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

type AlertStatus string

const (
	AlertFiring   AlertStatus = "firing"
	AlertResolved AlertStatus = "resolved"
)

type AlertKey struct {
	TenantID    string
	OrgID       string
	SourceID    string
	Fingerprint string
	StartsAt    time.Time
	Status      AlertStatus
}

type AlertEvent struct {
	ID         string
	Key        AlertKey
	ServiceRef string
	AlertName  string
	Labels     map[string]string
	TaskID     string
	ReceivedAt time.Time
}

func NewAlertEvent(id string, key AlertKey, serviceRef, alertName string, labels map[string]string, taskID string, receivedAt time.Time) (AlertEvent, error) {
	if id == "" || key.TenantID == "" || key.OrgID == "" || key.SourceID == "" || key.Fingerprint == "" || key.StartsAt.IsZero() || (key.Status != AlertFiring && key.Status != AlertResolved) || strings.TrimSpace(serviceRef) == "" || strings.TrimSpace(alertName) == "" || len(labels) == 0 || len(labels) > 24 || receivedAt.IsZero() {
		return AlertEvent{}, common.NewError(common.InvalidArgument, "alert event is invalid", false)
	}
	if key.Status == AlertFiring && taskID == "" {
		return AlertEvent{}, common.NewError(common.InvalidArgument, "firing alert requires an Incident Task", false)
	}
	cloned := make(map[string]string, len(labels))
	for label, value := range labels {
		if strings.TrimSpace(label) == "" || len(value) > 200 {
			return AlertEvent{}, common.NewError(common.InvalidArgument, "alert labels are invalid", false)
		}
		cloned[label] = value
	}
	key.StartsAt = key.StartsAt.UTC()
	return AlertEvent{ID: id, Key: key, ServiceRef: serviceRef, AlertName: alertName, Labels: cloned, TaskID: taskID, ReceivedAt: receivedAt.UTC()}, nil
}
