package jsonl

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"mini-torchbearing.local/services/assistant-mcp/internal/ports/executionaudit"
)

func TestStoreAppendsCompleteConcurrentJSONLRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit", "execution.jsonl")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Date(2026, 7, 16, 15, 0, 0, 0, time.UTC)
	var group sync.WaitGroup
	for index := range 20 {
		group.Add(1)
		go func() {
			defer group.Done()
			record := executionaudit.Record{ID: time.Now().String(), TenantID: "org:1", OrgID: "1", TaskID: "task", ApprovalID: "approval", IntentDigest: "sha256:digest", OperationID: "operation", Phase: "execute", Outcome: "succeeded", OccurredAt: now.Add(time.Duration(index) * time.Millisecond)}
			if err := store.Append(context.Background(), record); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytesLines(contents)
	if len(lines) != 20 {
		t.Fatalf("lines=%d", len(lines))
	}
	for _, line := range lines {
		var record executionaudit.Record
		if json.Unmarshal(line, &record) != nil || record.TaskID != "task" {
			t.Fatalf("invalid record: %s", line)
		}
	}
}

func bytesLines(value []byte) [][]byte {
	var result [][]byte
	start := 0
	for index, item := range value {
		if item == '\n' {
			result = append(result, value[start:index])
			start = index + 1
		}
	}
	return result
}
