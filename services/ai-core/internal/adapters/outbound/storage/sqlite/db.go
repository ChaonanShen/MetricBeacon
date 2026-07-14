// Package sqlite implements the AI Core persistence ports using SQLite.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"mini-torchbearing.local/services/ai-core/internal/domain/common"
	_ "modernc.org/sqlite"
)

const defaultBusyTimeoutMilliseconds = 5_000

// Open opens a single-owner AI Core SQLite database, configures every connection
// with the required SQLite pragmas, and applies all known migrations.
func Open(ctx context.Context, path string) (*Store, error) {
	if strings.TrimSpace(path) == "" || path == ":memory:" {
		return nil, common.NewError(common.InvalidArgument, "a persistent SQLite database path is required", false)
	}
	if !strings.HasPrefix(path, "file:") {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, mapError(err)
		}
	}

	db, err := sql.Open("sqlite", sqliteDSN(path))
	if err != nil {
		return nil, mapError(err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	store := &Store{db: db, writeMu: &sync.Mutex{}}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.Health(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func sqliteDSN(path string) string {
	if strings.HasPrefix(path, "file:") {
		separator := "?"
		if strings.Contains(path, "?") {
			separator = "&"
		}
		return path + separator + pragmaQuery()
	}
	uri := &url.URL{Scheme: "file", Path: path, RawQuery: pragmaQuery()}
	return uri.String()
}

func pragmaQuery() string {
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", defaultBusyTimeoutMilliseconds))
	values.Add("_pragma", "journal_mode(WAL)")
	return values.Encode()
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	var domainErr *common.DomainError
	if errors.As(err, &domainErr) {
		return domainErr
	}
	if errors.Is(err, sql.ErrNoRows) {
		return common.NewError(common.ResourceNotFound, "resource was not found", false)
	}

	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "unique constraint"), strings.Contains(message, "primary key"):
		return common.NewError(common.ResourceConflict, "resource already exists", false)
	case strings.Contains(message, "foreign key constraint"):
		return common.NewError(common.ResourceConflict, "referenced resource does not exist", false)
	case strings.Contains(message, "check constraint"), strings.Contains(message, "not null constraint"), strings.Contains(message, "json_valid"):
		return common.NewError(common.InvalidArgument, "stored value violates a persistence constraint", false)
	case strings.Contains(message, "database is locked"), strings.Contains(message, "database is busy"):
		return common.NewError(common.DependencyUnavailable, "SQLite database is temporarily unavailable", true)
	default:
		return common.NewError(common.InternalError, "SQLite persistence operation failed", true)
	}
}
