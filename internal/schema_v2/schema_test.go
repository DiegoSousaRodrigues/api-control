package schema_v2

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestStagingBaselineIsOutsideProductionMigrationGlob(t *testing.T) {
	if strings.Contains(upPath, "internal/migrations") || !strings.HasPrefix(upPath, "baseline/") {
		t.Fatalf("unexpected staging up path %q", upPath)
	}
}
func TestBaselineContainsApprovedTablesAndFinancialConstraints(t *testing.T) {
	contents, err := UpSQL()
	if err != nil {
		t.Fatal(err)
	}
	sqlText := string(contents)
	for _, required := range []string{
		"CREATE TABLE users",
		"CREATE TABLE clients",
		"CREATE TABLE products",
		"CREATE TABLE billing_periods",
		"CREATE TABLE invoices",
		"CREATE TABLE invoice_items",
		"CREATE TABLE payments",
		"CREATE TABLE payment_allocations",
		"NUMERIC(14,2)",
		"NUMERIC(15,2)",
		"GENERATED ALWAYS AS",
		"ON DELETE RESTRICT",
		"FOREIGN KEY (payment_id, client_id)",
		"FOREIGN KEY (invoice_id, client_id)",
		"WHERE status = 'issued'",
		"WHERE status = 'active'",
	} {
		if !strings.Contains(sqlText, required) {
			t.Fatalf("baseline missing %q", required)
		}
	}
	for _, forbidden := range []string{"DOUBLE PRECISION", "client_account_entry", `CREATE TABLE "order"`, "CREATE TABLE schema_migrations"} {
		if strings.Contains(sqlText, forbidden) {
			t.Fatalf("baseline contains legacy construct %q", forbidden)
		}
	}
}

func TestBaselineDownDropsTablesInForeignKeyOrder(t *testing.T) {
	contents, err := DownSQL()
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	order := []string{
		"DROP TABLE IF EXISTS payment_allocations",
		"DROP TABLE IF EXISTS payments",
		"DROP TABLE IF EXISTS invoice_items",
		"DROP TABLE IF EXISTS invoices",
		"DROP TABLE IF EXISTS billing_periods",
		"DROP TABLE IF EXISTS products",
		"DROP TABLE IF EXISTS clients",
		"DROP TABLE IF EXISTS users",
	}
	previous := -1
	for _, statement := range order {
		index := strings.Index(text, statement)
		if index <= previous {
			t.Fatalf("%q is missing or out of order", statement)
		}
		previous = index
	}
}

func TestApplyUpIsAtomic(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	contents, err := UpSQL()
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(string(contents))).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	if err := ApplyUp(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyUpRollsBackOnFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	contents, err := UpSQL()
	if err != nil {
		t.Fatal(err)
	}
	expected := errors.New("ddl failed")
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(string(contents))).WillReturnError(expected)
	mock.ExpectRollback()
	if err := ApplyUp(context.Background(), db); !errors.Is(err, expected) {
		t.Fatalf("error = %v, want %v", err, expected)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyDownIsAtomic(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	contents, err := DownSQL()
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(string(contents))).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	if err := ApplyDown(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
