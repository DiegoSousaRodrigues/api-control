package schema_v2

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

const (
	postgresTestDSNEnvironment   = "V2_SCHEMA_TEST_DATABASE_URL"
	postgresTestGuardEnvironment = "V2_SCHEMA_TEST_ALLOW_DISPOSABLE"
)

var postgresIdentifier = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// TestBaselineAgainstDisposablePostgres is intentionally opt-in. It creates and
// destroys only a uniquely named schema inside the explicitly authorized test
// database; it never changes the current development schema.
func TestBaselineAgainstDisposablePostgres(t *testing.T) {
	dsn := os.Getenv(postgresTestDSNEnvironment)
	if dsn == "" {
		t.Skip(postgresTestDSNEnvironment + " is not configured")
	}
	if os.Getenv(postgresTestGuardEnvironment) != "yes" {
		t.Fatalf("%s=yes is required for the disposable PostgreSQL test", postgresTestGuardEnvironment)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	adminConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid disposable PostgreSQL test configuration")
	}
	adminDB := stdlib.OpenDB(*adminConfig)
	defer adminDB.Close()
	if err := adminDB.PingContext(ctx); err != nil {
		t.Fatal("cannot connect to disposable PostgreSQL test database")
	}

	schemaName := fmt.Sprintf("control_v2_%d", time.Now().UnixNano())
	if !postgresIdentifier.MatchString(schemaName) {
		t.Fatalf("unsafe generated schema name %q", schemaName)
	}
	if _, err := adminDB.ExecContext(ctx, `CREATE SCHEMA `+schemaName); err != nil {
		t.Fatalf("create isolated test schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, err := adminDB.ExecContext(cleanupCtx, `DROP SCHEMA `+schemaName+` CASCADE`); err != nil {
			t.Errorf("drop isolated test schema: %v", err)
		}
	})

	testConfig := adminConfig.Copy()
	testConfig.RuntimeParams["search_path"] = schemaName
	testDB := stdlib.OpenDB(*testConfig)
	defer testDB.Close()
	if err := testDB.PingContext(ctx); err != nil {
		t.Fatal("cannot enter isolated PostgreSQL test schema")
	}

	if err := ApplyUp(ctx, testDB); err != nil {
		t.Fatalf("apply baseline to empty schema: %v", err)
	}
	assertBaselineTables(t, ctx, testDB)
	assertBaselineConstraints(t, ctx, testDB)
	if err := ApplyDown(ctx, testDB); err != nil {
		t.Fatalf("apply baseline down: %v", err)
	}
	assertBaselineRemoved(t, ctx, testDB)
}

func assertBaselineTables(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	for _, table := range []string{
		"users", "clients", "products", "billing_periods", "invoices",
		"invoice_items", "payments", "payment_allocations",
	} {
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil {
			t.Fatalf("inspect table %s: %v", table, err)
		}
		if !exists {
			t.Fatalf("baseline did not create table %s", table)
		}
	}
}

func assertBaselineConstraints(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO products (name, purchase_price, sale_price)
		VALUES ('invalid', -0.01, 1.00)
	`); err == nil {
		t.Fatal("products accepted a negative purchase price")
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO billing_periods (period_start) VALUES ('2026-08-02')
	`); err == nil {
		t.Fatal("billing_periods accepted a date other than the first day of the month")
	}

	var clientOne, clientTwo, period, product, invoice, payment int64
	if err := db.QueryRowContext(ctx, `
		INSERT INTO clients
			(name, document, phone, street, neighborhood, address_number, address_type)
		VALUES ('Client one', '111', '1', 'Street', 'Center', '1', 'home')
		RETURNING id
	`).Scan(&clientOne); err != nil {
		t.Fatalf("insert first constraint fixture client: %v", err)
	}
	if err := db.QueryRowContext(ctx, `
		INSERT INTO clients
			(name, document, phone, street, neighborhood, address_number, address_type)
		VALUES ('Client two', '222', '2', 'Street', 'Center', '2', 'home')
		RETURNING id
	`).Scan(&clientTwo); err != nil {
		t.Fatalf("insert second constraint fixture client: %v", err)
	}
	if err := db.QueryRowContext(ctx, `
		INSERT INTO billing_periods (period_start) VALUES ('2026-08-01') RETURNING id
	`).Scan(&period); err != nil {
		t.Fatalf("insert constraint fixture billing period: %v", err)
	}
	if err := db.QueryRowContext(ctx, `
		INSERT INTO products (name, purchase_price, sale_price)
		VALUES ('Product', 1.00, 2.00) RETURNING id
	`).Scan(&product); err != nil {
		t.Fatalf("insert constraint fixture product: %v", err)
	}
	if err := db.QueryRowContext(ctx, `
		INSERT INTO invoices
			(client_id, billing_period_id, status, account_balance_before_issue, products_total, account_balance_after_charge)
		VALUES ($1, $2, 'issued', 0.00, 2.00, 2.00) RETURNING id
	`, clientOne, period).Scan(&invoice); err != nil {
		t.Fatalf("insert constraint fixture invoice: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO invoice_items
			(invoice_id, product_id, product_name, quantity, unit_purchase_price, unit_sale_price)
		VALUES ($1, $2, 'Product', 1, 1.00, 2.00)
	`, invoice, product); err != nil {
		t.Fatalf("insert valid invoice item: %v", err)
	}
	if err := db.QueryRowContext(ctx, `
		INSERT INTO payments (client_id, amount, effective_date, status)
		VALUES ($1, 2.00, '2026-08-15', 'posted') RETURNING id
	`, clientTwo).Scan(&payment); err != nil {
		t.Fatalf("insert constraint fixture payment: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO payment_allocations (client_id, payment_id, invoice_id, amount, status)
		VALUES ($1, $2, $3, 2.00, 'active')
	`, clientOne, payment, invoice); err == nil {
		t.Fatal("payment allocation accepted a payment and invoice from different clients")
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM clients WHERE id = $1`, clientOne); err == nil {
		t.Fatal("client foreign key did not restrict deletion")
	}
}

func assertBaselineRemoved(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT to_regclass('users') IS NOT NULL`).Scan(&exists); err != nil {
		t.Fatalf("inspect down result: %v", err)
	}
	if exists {
		t.Fatal("baseline down left target tables behind")
	}
}
