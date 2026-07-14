package session

import (
	"strings"
	"time"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	ID        string
	TenantID  string
	SessionID string
	TaskID    string
	Role      Role
	Content   string
	CreatedAt time.Time
}

func NewMessage(id, tenantID, sessionID, taskID string, role Role, content string, now time.Time) (Message, error) {
	if id == "" || tenantID == "" || sessionID == "" || taskID == "" || strings.TrimSpace(content) == "" {
		return Message{}, common.NewError(common.InvalidArgument, "message id, tenant, session, task and content are required", false)
	}
	if role != RoleUser && role != RoleAssistant {
		return Message{}, common.NewError(common.InvalidArgument, "message role is invalid", false)
	}
	return Message{ID: id, TenantID: tenantID, SessionID: sessionID, TaskID: taskID, Role: role, Content: content, CreatedAt: now.UTC()}, nil
}
