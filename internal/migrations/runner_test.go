package database

import (
	"context"
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestEmbeddedMigrationsContainVersionsOneAndTwoInOrder(t *testing.T) {
	items, err := loadMigrations(migrationFiles)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 2 || items[0].version != 1 || items[1].version != 2 {
		t.Fatalf("versions = %+v, want 1 then 2", items)
	}
	for _, item := range items {
		if containsTransactionControl(item.sql) {
			t.Fatalf("%s contains transaction control", item.id)
		}
	}
}

func TestRunMigrationsAppliesPendingMigrationAtomicallyAndSkipsApplied(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec(`SELECT pg_advisory_lock`).WithArgs(migrationAdvisoryLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS schema_migrations`).WillReturnResult(sqlmock.NewResult(0, 0))
	// sql.ErrNoRows is what database/sql returns for an empty result set.
	mock.ExpectQuery(`SELECT migration_id FROM schema_migrations`).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"migration_id"}))
	mock.ExpectBegin()
	mock.ExpectExec(`CREATE TABLE first`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`INSERT INTO schema_migrations`).WithArgs(int64(1), "000001_first.up.sql").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(`SELECT migration_id FROM schema_migrations`).WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows([]string{"migration_id"}).AddRow("000002_second.up.sql"))
	mock.ExpectExec(`SELECT pg_advisory_unlock`).WithArgs(migrationAdvisoryLockID).WillReturnResult(sqlmock.NewResult(0, 1))

	err = runMigrations(context.Background(), db, testMigrationFS())
	if err != nil {
		t.Fatalf("runMigrations error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRunMigrationsRollsBackAndDoesNotRecordFailedMigration(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	migrationErr := errors.New("ddl failed")
	mock.ExpectExec(`SELECT pg_advisory_lock`).WithArgs(migrationAdvisoryLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS schema_migrations`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT migration_id FROM schema_migrations`).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"migration_id"}))
	mock.ExpectBegin()
	mock.ExpectExec(`CREATE TABLE first`).WillReturnError(migrationErr)
	mock.ExpectRollback()
	mock.ExpectExec(`SELECT pg_advisory_unlock`).WithArgs(migrationAdvisoryLockID).WillReturnResult(sqlmock.NewResult(0, 1))

	err = runMigrations(context.Background(), db, fstest.MapFS{"000001_first.up.sql": {Data: []byte("CREATE TABLE first(id int)")}})
	if !errors.Is(err, migrationErr) {
		t.Fatalf("error = %v, want %v", err, migrationErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMigrationsRejectsDuplicateVersion(t *testing.T) {
	_, err := loadMigrations(fstest.MapFS{
		"000001_first.up.sql": {Data: []byte("SELECT 1")},
		"000001_other.up.sql": {Data: []byte("SELECT 2")},
	})
	if err == nil {
		t.Fatal("duplicate version error = nil")
	}
}

func testMigrationFS() fs.FS {
	return fstest.MapFS{
		"000002_second.up.sql": {Data: []byte("CREATE TABLE second(id int)")},
		"000001_first.up.sql":  {Data: []byte("CREATE TABLE first(id int)")},
	}
}

func containsTransactionControl(sql string) bool {
	return migrationTransactionPattern.MatchString(sql)
}
