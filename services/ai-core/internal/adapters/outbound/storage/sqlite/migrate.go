package sqlite

import (
	"context"
	"database/sql"
	"time"

	migrations "mini-torchbearing.local/services/ai-core/migrations/sqlite"
)

type migration struct {
	version int
	sql     string
}

var allMigrations = []migration{{version: 1, sql: migrations.Initial}}

func (s *Store) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return mapError(err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`); err != nil {
		return mapError(err)
	}
	for _, item := range allMigrations {
		var applied int
		err := tx.QueryRowContext(ctx, "SELECT 1 FROM schema_migrations WHERE version = ?", item.version).Scan(&applied)
		switch {
		case err == nil:
			continue
		case err != sql.ErrNoRows:
			return mapError(err)
		}
		if _, err := tx.ExecContext(ctx, item.sql); err != nil {
			return mapError(err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)", item.version, storageTimestamp(nowUTC())); err != nil {
			return mapError(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return mapError(err)
	}
	return nil
}

func nowUTC() time.Time { return time.Now().UTC() }
