package schema_v2

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"regexp"
)

// Files is deliberately outside internal/migrations, so the production runner
// cannot discover or execute this staging baseline before the coordinated cut.
//
//go:embed baseline/*.sql
var Files embed.FS

const (
	upPath   = "baseline/000001_initial_schema.up.sql"
	downPath = "baseline/000001_initial_schema.down.sql"
)

var transactionControlPattern = regexp.MustCompile(`(?im)^\s*(BEGIN|COMMIT)\s*;`)

func UpSQL() ([]byte, error)   { return readSQL(upPath) }
func DownSQL() ([]byte, error) { return readSQL(downPath) }

func ApplyUp(ctx context.Context, db *sql.DB) error {
	return apply(ctx, db, upPath)
}

func ApplyDown(ctx context.Context, db *sql.DB) error {
	return apply(ctx, db, downPath)
}

func readSQL(path string) ([]byte, error) {
	contents, err := Files.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read staging schema %s: %w", path, err)
	}
	if transactionControlPattern.Match(contents) {
		return nil, fmt.Errorf("staging schema %s must not manage its own transaction", path)
	}
	return contents, nil
}

func apply(ctx context.Context, db *sql.DB, path string) (resultErr error) {
	if db == nil {
		return errors.New("staging schema database is required")
	}
	contents, err := readSQL(path)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin staging schema %s: %w", path, err)
	}
	defer func() {
		if resultErr != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.ExecContext(ctx, string(contents)); err != nil {
		return fmt.Errorf("apply staging schema %s: %w", path, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit staging schema %s: %w", path, err)
	}
	return nil
}
