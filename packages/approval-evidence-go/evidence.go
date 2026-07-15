// Package approvalevidence signs and verifies short-lived, exact-scope
// authorization passed from AI Core to a typed write adapter.
package approvalevidence

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	Version     = 1
	MaxLifetime = 60 * time.Second
	clockSkew   = 5 * time.Second
)

var (
	ErrInvalid = errors.New("approval evidence is invalid")
	ErrExpired = errors.New("approval evidence is expired")
)

type Claims struct {
	Version         int       `json:"version"`
	TenantID        string    `json:"tenantId"`
	OrgID           string    `json:"orgId"`
	TaskID          string    `json:"taskId"`
	ApprovalID      string    `json:"approvalId"`
	IntentDigest    string    `json:"intentDigest"`
	CapabilityID    string    `json:"capabilityId"`
	ServiceRef      string    `json:"serviceRef"`
	InstanceEpoch   string    `json:"instanceEpoch"`
	ExpectedVersion int       `json:"expectedVersion"`
	OperationID     string    `json:"operationId"`
	IssuedAt        time.Time `json:"issuedAt"`
	ExpiresAt       time.Time `json:"expiresAt"`
}

// ExpectedScope contains every claim visible at the assistant-mcp boundary.
// TaskID remains signed and non-empty but is intentionally not duplicated in
// the write-tool input or exposed to an Agent.
type ExpectedScope struct {
	TenantID        string
	OrgID           string
	ApprovalID      string
	IntentDigest    string
	CapabilityID    string
	ServiceRef      string
	InstanceEpoch   string
	ExpectedVersion int
	OperationID     string
}

type Codec struct{ key []byte }

func New(key []byte) (*Codec, error) {
	if len(key) < 32 {
		return nil, ErrInvalid
	}
	return &Codec{key: append([]byte(nil), key...)}, nil
}

func (c *Codec) Sign(claims Claims) (string, error) {
	claims.IssuedAt, claims.ExpiresAt = claims.IssuedAt.UTC(), claims.ExpiresAt.UTC()
	if !validClaims(claims) {
		return "", ErrInvalid
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", ErrInvalid
	}
	signature := c.signature(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (c *Codec) Verify(token string, expected ExpectedScope, now time.Time) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || !validExpected(expected) {
		return Claims{}, ErrInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || len(payload) > 4096 {
		return Claims{}, ErrInvalid
	}
	provided, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(provided, c.signature(payload)) {
		return Claims{}, ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var claims Claims
	if err := decoder.Decode(&claims); err != nil || decoder.More() || !validClaims(claims) || !matches(claims, expected) {
		return Claims{}, ErrInvalid
	}
	now = now.UTC()
	if now.Before(claims.IssuedAt.Add(-clockSkew)) {
		return Claims{}, ErrInvalid
	}
	if !now.Before(claims.ExpiresAt) {
		return Claims{}, ErrExpired
	}
	return claims, nil
}

func (c *Codec) signature(payload []byte) []byte {
	mac := hmac.New(sha256.New, c.key)
	_, _ = mac.Write(payload)
	return mac.Sum(nil)
}

func validClaims(value Claims) bool {
	return value.Version == Version && value.TenantID != "" && value.OrgID != "" && value.TaskID != "" && value.ApprovalID != "" && validDigest(value.IntentDigest) && value.CapabilityID == "order_service.restore_worker_concurrency" && value.ServiceRef == "order-demo" && value.InstanceEpoch != "" && value.ExpectedVersion >= 1 && value.OperationID != "" && !value.IssuedAt.IsZero() && value.ExpiresAt.After(value.IssuedAt) && value.ExpiresAt.Sub(value.IssuedAt) <= MaxLifetime
}

func validExpected(value ExpectedScope) bool {
	return value.TenantID != "" && value.OrgID != "" && value.ApprovalID != "" && validDigest(value.IntentDigest) && value.CapabilityID == "order_service.restore_worker_concurrency" && value.ServiceRef == "order-demo" && value.InstanceEpoch != "" && value.ExpectedVersion >= 1 && value.OperationID != ""
}

func matches(claims Claims, expected ExpectedScope) bool {
	return claims.TenantID == expected.TenantID && claims.OrgID == expected.OrgID && claims.ApprovalID == expected.ApprovalID && claims.IntentDigest == expected.IntentDigest && claims.CapabilityID == expected.CapabilityID && claims.ServiceRef == expected.ServiceRef && claims.InstanceEpoch == expected.InstanceEpoch && claims.ExpectedVersion == expected.ExpectedVersion && claims.OperationID == expected.OperationID
}

func validDigest(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, char := range value[7:] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}
