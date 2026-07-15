package jsonl

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"mini-torchbearing.local/services/assistant-mcp/internal/ports/executionaudit"
)

type Store struct {
	mu   sync.Mutex
	file *os.File
}

func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("execution audit path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &Store{file: file}, nil
}

func (s *Store) Append(ctx context.Context, record executionaudit.Record) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.file == nil || record.ID == "" || record.TenantID == "" || record.OrgID == "" || record.TaskID == "" || record.ApprovalID == "" || record.IntentDigest == "" || record.OperationID == "" || (record.Phase != "execute" && record.Phase != "reconcile") || (record.Outcome != "authorized" && record.Outcome != "succeeded" && record.Outcome != "failed") || record.OccurredAt.IsZero() {
		return fmt.Errorf("execution audit record is invalid")
	}
	record.OccurredAt = record.OccurredAt.UTC()
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.file.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return s.file.Sync()
}

func (s *Store) Close() error {
	if s == nil || s.file == nil {
		return nil
	}
	return s.file.Close()
}
