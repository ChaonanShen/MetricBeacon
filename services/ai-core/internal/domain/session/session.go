package session

import (
	"strings"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

type Status string

const StatusActive Status = "active"

type AnalysisSession struct {
	ID        string
	TenantID  string
	Title     string
	Status    Status
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
	Version   int64
}

func New(id, tenantID, title, createdBy string, now time.Time) (AnalysisSession, error) {
	if id == "" || tenantID == "" || createdBy == "" || strings.TrimSpace(title) == "" {
		return AnalysisSession{}, common.NewError(common.InvalidArgument, "session id, tenant, title and creator are required", false)
	}
	now = now.UTC()
	return AnalysisSession{ID: id, TenantID: tenantID, Title: title, Status: StatusActive, CreatedBy: createdBy, CreatedAt: now, UpdatedAt: now, Version: 1}, nil
}
