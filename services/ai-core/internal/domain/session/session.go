package session

import (
	"strings"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

type Status string
type Kind string

const StatusActive Status = "active"

const (
	KindPrivate     Kind = "private"
	KindOrgIncident Kind = "org_incident"
)

type AnalysisSession struct {
	ID        string
	TenantID  string
	OrgID     string
	Kind      Kind
	Title     string
	Status    Status
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
	Version   int64
}

func NewPrivate(id, tenantID, orgID, title, createdBy string, now time.Time) (AnalysisSession, error) {
	return newSession(id, tenantID, orgID, KindPrivate, title, createdBy, now)
}

func NewIncident(id, tenantID, orgID, title, createdBy string, now time.Time) (AnalysisSession, error) {
	return newSession(id, tenantID, orgID, KindOrgIncident, title, createdBy, now)
}

func newSession(id, tenantID, orgID string, kind Kind, title, createdBy string, now time.Time) (AnalysisSession, error) {
	if id == "" || tenantID == "" || orgID == "" || createdBy == "" || strings.TrimSpace(title) == "" {
		return AnalysisSession{}, common.NewError(common.InvalidArgument, "session id, tenant, organization, title and creator are required", false)
	}
	if kind != KindPrivate && kind != KindOrgIncident {
		return AnalysisSession{}, common.NewError(common.InvalidArgument, "session kind is invalid", false)
	}
	now = now.UTC()
	return AnalysisSession{ID: id, TenantID: tenantID, OrgID: orgID, Kind: kind, Title: title, Status: StatusActive, CreatedBy: createdBy, CreatedAt: now, UpdatedAt: now, Version: 1}, nil
}

func (s *AnalysisSession) Touch(now time.Time) error {
	now = now.UTC()
	if now.Before(s.UpdatedAt) {
		return common.NewError(common.InvalidStateTransition, "session activity time cannot move backwards", false)
	}
	s.UpdatedAt = now
	s.Version++
	return nil
}
