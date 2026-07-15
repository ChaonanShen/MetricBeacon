package approvalevidence

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestEvidenceRoundTripBindsExactScope(t *testing.T) {
	now := time.Date(2026, 7, 16, 13, 0, 0, 0, time.UTC)
	codec := testCodec(t)
	claims := testClaims(now)
	token, err := codec.Sign(claims)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := codec.Verify(token, testExpected(), now.Add(30*time.Second))
	if err != nil || verified.TaskID != claims.TaskID || verified.ExpiresAt != claims.ExpiresAt {
		t.Fatalf("verified=%#v err=%v", verified, err)
	}

	for name, mutate := range map[string]func(ExpectedScope) ExpectedScope{
		"tenant":    func(value ExpectedScope) ExpectedScope { value.TenantID = "org:2"; return value },
		"approval":  func(value ExpectedScope) ExpectedScope { value.ApprovalID = "approval-2"; return value },
		"intent":    func(value ExpectedScope) ExpectedScope { value.IntentDigest = digest("b"); return value },
		"epoch":     func(value ExpectedScope) ExpectedScope { value.InstanceEpoch = "epoch-2"; return value },
		"version":   func(value ExpectedScope) ExpectedScope { value.ExpectedVersion++; return value },
		"operation": func(value ExpectedScope) ExpectedScope { value.OperationID = "operation-2"; return value },
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := codec.Verify(token, mutate(testExpected()), now); !errors.Is(err, ErrInvalid) {
				t.Fatalf("scope mismatch error = %v", err)
			}
		})
	}
}

func TestEvidenceRejectsTamperExpiryFutureAndLongLifetime(t *testing.T) {
	now := time.Date(2026, 7, 16, 13, 0, 0, 0, time.UTC)
	codec := testCodec(t)
	token, _ := codec.Sign(testClaims(now))
	if _, err := codec.Verify(token+"x", testExpected(), now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("tamper error = %v", err)
	}
	if _, err := codec.Verify(token, testExpected(), now.Add(MaxLifetime)); !errors.Is(err, ErrExpired) {
		t.Fatalf("expiry error = %v", err)
	}
	if _, err := codec.Verify(token, testExpected(), now.Add(-6*time.Second)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("future issue error = %v", err)
	}
	claims := testClaims(now)
	claims.ExpiresAt = now.Add(MaxLifetime + time.Nanosecond)
	if _, err := codec.Sign(claims); !errors.Is(err, ErrInvalid) {
		t.Fatalf("long lifetime error = %v", err)
	}
}

func TestEvidenceRejectsShortKeyAndUnknownClaims(t *testing.T) {
	if _, err := New([]byte("short")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("short key error = %v", err)
	}
	now := time.Date(2026, 7, 16, 13, 0, 0, 0, time.UTC)
	codec := testCodec(t)
	payload := []byte(`{"version":1,"tenantId":"org:1","orgId":"1","taskId":"task-1","approvalId":"approval-1","intentDigest":"` + digest("a") + `","capabilityId":"order_service.restore_worker_concurrency","serviceRef":"order-demo","instanceEpoch":"epoch-1","expectedVersion":2,"operationId":"operation-1","issuedAt":"2026-07-16T13:00:00Z","expiresAt":"2026-07-16T13:01:00Z","unknown":true}`)
	token := base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(codec.signature(payload))
	if _, err := codec.Verify(token, testExpected(), now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown claim error = %v", err)
	}
}

func testCodec(t *testing.T) *Codec {
	t.Helper()
	codec, err := New([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	return codec
}

func testClaims(now time.Time) Claims {
	return Claims{Version: Version, TenantID: "org:1", OrgID: "1", TaskID: "task-1", ApprovalID: "approval-1", IntentDigest: digest("a"), CapabilityID: "order_service.restore_worker_concurrency", ServiceRef: "order-demo", InstanceEpoch: "epoch-1", ExpectedVersion: 2, OperationID: "operation-1", IssuedAt: now, ExpiresAt: now.Add(MaxLifetime)}
}

func testExpected() ExpectedScope {
	return ExpectedScope{TenantID: "org:1", OrgID: "1", ApprovalID: "approval-1", IntentDigest: digest("a"), CapabilityID: "order_service.restore_worker_concurrency", ServiceRef: "order-demo", InstanceEpoch: "epoch-1", ExpectedVersion: 2, OperationID: "operation-1"}
}

func digest(char string) string { return "sha256:" + strings.Repeat(char, 64) }
