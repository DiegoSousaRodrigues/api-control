package database

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"github.com/api-control/internal/domain"
)

const migrationAdvisoryLockID int64 = 739204618521

//go:embed *.up.sql
var migrationFiles embed.FS

var Migrations IMigrationRunner = &migrationRunner{}

type IMigrationRunner interface {
	Run(context.Context) error
}

type migrationRunner struct {
	db domain.BaseRepository
}

type migration struct {
	version int64
	id      string
	sql     string
}

var migrationNamePattern = regexp.MustCompile(`^(\d+)_.*\.up\.sql$`)
var migrationTransactionPattern = regexp.MustCompile(`(?im)^\s*(BEGIN|COMMIT)\s*;`)

func (runner *migrationRunner) Run(ctx context.Context) error {
	gormDB := runner.db.PSQL()
	sqlDB, err := gormDB.DB()
	if err != nil {
		return fmt.Errorf("get migration database: %w", err)
	}
	return runMigrations(ctx, sqlDB, migrationFiles)
}

func runMigrations(ctx context.Context, db *sql.DB, files fs.FS) (resultErr error) {
	migrations, err := loadMigrations(files)
	if err != nil {
		return err
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, migrationAdvisoryLockID); err != nil {
		return fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	defer func() {
		_, unlockErr := conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationAdvisoryLockID)
		if resultErr == nil && unlockErr != nil {
			resultErr = fmt.Errorf("release migration advisory lock: %w", unlockErr)
		}
	}()

	if _, err := conn.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT PRIMARY KEY,
    migration_id TEXT NOT NULL UNIQUE,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	for _, item := range migrations {
		var appliedID string
		err := conn.QueryRowContext(ctx, `SELECT migration_id FROM schema_migrations WHERE version = $1`, item.version).Scan(&appliedID)
		switch {
		case err == nil:
			if appliedID != item.id {
				return fmt.Errorf("migration version %d already belongs to %q, not %q", item.version, appliedID, item.id)
			}
			continue
		case !errors.Is(err, sql.ErrNoRows):
			return fmt.Errorf("check migration %s: %w", item.id, err)
		}

		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", item.id, err)
		}
		if _, err := tx.ExecContext(ctx, item.sql); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("execute migration %s: %w", item.id, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, migration_id) VALUES ($1, $2)`, item.version, item.id); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", item.id, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", item.id, err)
		}
	}

	return nil
}

func loadMigrations(files fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}

	items := make([]migration, 0)
	versions := make(map[int64]string)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" || !migrationNamePattern.MatchString(entry.Name()) {
			continue
		}
		matches := migrationNamePattern.FindStringSubmatch(entry.Name())
		version, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil || version <= 0 {
			return nil, fmt.Errorf("invalid migration version in %q", entry.Name())
		}
		if previous, exists := versions[version]; exists {
			return nil, fmt.Errorf("duplicate migration version %d in %q and %q", version, previous, entry.Name())
		}
		contents, err := fs.ReadFile(files, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		if migrationTransactionPattern.Match(contents) {
			return nil, fmt.Errorf("migration %s must not manage its own transaction", entry.Name())
		}
		versions[version] = entry.Name()
		items = append(items, migration{version: version, id: entry.Name(), sql: string(contents)})
	}
	if len(items) == 0 {
		return nil, errors.New("no embedded up migrations found")
	}
	sort.Slice(items, func(i, j int) bool { return items[i].version < items[j].version })
	return items, nil
}
