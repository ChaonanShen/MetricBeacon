package session

import (
	"testing"
	"time"
)

func TestTriggerMessageIsExplicitAndValidated(t *testing.T) {
	value, err := NewMessage("message_1", "org:1", "session_1", "task_1", RoleTrigger, "OrderQueueBacklog firing", time.Now())
	if err != nil || value.Role != RoleTrigger {
		t.Fatalf("trigger Message = %#v, %v", value, err)
	}
	if _, err := NewMessage("message_2", "org:1", "session_1", "task_1", Role("system"), "hidden ground truth", time.Now()); err == nil {
		t.Fatal("uncontracted system role unexpectedly succeeded")
	}
}
