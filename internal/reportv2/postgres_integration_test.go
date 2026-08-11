package reportv2_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/api-control/internal/reportv2"
	schemav2 "github.com/api-control/internal/schema_v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var safeSchema = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// TestPostgresClientBalanceV2 is opt-in and only creates a unique schema in a
// database explicitly authorized as disposable.
func TestPostgresClientBalanceV2(t *testing.T) {
	dsn := os.Getenv("V2_SCHEMA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("V2_SCHEMA_TEST_DATABASE_URL is not configured")
	}
	if os.Getenv("V2_SCHEMA_TEST_ALLOW_DISPOSABLE") != "yes" {
		t.Fatal("V2_SCHEMA_TEST_ALLOW_DISPOSABLE=yes is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid disposable PostgreSQL configuration")
	}
	admin := stdlib.OpenDB(*config)
	defer admin.Close()
	schema := fmt.Sprintf("control_report_v2_%d", time.Now().UnixNano())
	if !safeSchema.MatchString(schema) {
		t.Fatalf("unsafe schema %q", schema)
	}
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, err := admin.ExecContext(cleanup, `DROP SCHEMA `+schema+` CASCADE`); err != nil {
			t.Errorf("drop isolated schema: %v", err)
		}
	})
	testConfig := config.Copy()
	testConfig.RuntimeParams["search_path"] = schema
	testDB := stdlib.OpenDB(*testConfig)
	defer testDB.Close()
	if err := schemav2.ApplyUp(ctx, testDB); err != nil {
		t.Fatal(err)
	}

	clientID, mismatchedClientID := insertReportFixtures(t, ctx, testDB)
	database, err := gorm.Open(postgres.New(postgres.Config{Conn: testDB, PreferSimpleProtocol: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	store, _ := reportv2.NewStore(database)
	service, _ := reportv2.NewService(store)
	report, err := service.ClientBalance(ctx, clientID)
	if err != nil {
		t.Fatal(err)
	}
	if report.Totals.InvoiceCount != 1 || report.Totals.QuantityTotal != 2 || len(report.Months) != 1 ||
		!report.Totals.PurchaseTotal.Decimal().Equal(decimal.NewFromInt(120)) ||
		!report.Totals.SaleTotal.Decimal().Equal(decimal.NewFromInt(200)) ||
		!report.Totals.ProfitTotal.Decimal().Equal(decimal.NewFromInt(80)) {
		t.Fatalf("report = %#v", report)
	}
	if _, err := service.ClientBalance(ctx, mismatchedClientID); !errors.Is(err, reportv2.ErrInconsistentSnapshots) {
		t.Fatalf("mismatch error = %v", err)
	}
}

func insertReportFixtures(t *testing.T, ctx context.Context, db *sql.DB) (int64, int64) {
	t.Helper()
	clientID := insertClient(t, ctx, db, "Report client", "660001")
	mismatchedClientID := insertClient(t, ctx, db, "Mismatch client", "660002")
	var productID, augustID, septemberID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO products (name, purchase_price, sale_price)
        VALUES ('Historical product', 60.00, 100.00) RETURNING id`).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO billing_periods (period_start) VALUES ('2026-08-01') RETURNING id`).Scan(&augustID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO billing_periods (period_start) VALUES ('2026-09-01') RETURNING id`).Scan(&septemberID); err != nil {
		t.Fatal(err)
	}
	issuedID := insertInvoice(t, ctx, db, clientID, augustID, "issued", "200.00")
	if _, err := db.ExecContext(ctx, `INSERT INTO invoice_items
        (invoice_id, product_id, product_name, quantity, unit_purchase_price, unit_sale_price)
        VALUES ($1, $2, 'Historical product', 2, 60.00, 100.00)`, issuedID, productID); err != nil {
		t.Fatal(err)
	}
	canceledID := insertInvoice(t, ctx, db, clientID, septemberID, "canceled", "999.00")
	if _, err := db.ExecContext(ctx, `INSERT INTO invoice_items
        (invoice_id, product_id, product_name, quantity, unit_purchase_price, unit_sale_price)
        VALUES ($1, $2, 'Canceled product', 1, 10.00, 999.00)`, canceledID, productID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO payments (client_id, amount, effective_date, status)
        VALUES ($1, 1000.00, '2026-08-15', 'posted')`, clientID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE products SET purchase_price = 900.00, sale_price = 950.00 WHERE id = $1`, productID); err != nil {
		t.Fatal(err)
	}
	mismatchInvoiceID := insertInvoice(t, ctx, db, mismatchedClientID, augustID, "issued", "201.00")
	if _, err := db.ExecContext(ctx, `INSERT INTO invoice_items
        (invoice_id, product_id, product_name, quantity, unit_purchase_price, unit_sale_price)
        VALUES ($1, $2, 'Mismatch', 2, 60.00, 100.00)`, mismatchInvoiceID, productID); err != nil {
		t.Fatal(err)
	}
	return clientID, mismatchedClientID
}

func insertClient(t *testing.T, ctx context.Context, db *sql.DB, name, document string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRowContext(ctx, `INSERT INTO clients
        (name, document, phone, street, neighborhood, address_number, address_type)
        VALUES ($1, $2, '1', 'Street', 'Center', '1', 'home') RETURNING id`, name, document).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func insertInvoice(t *testing.T, ctx context.Context, db *sql.DB, clientID, periodID int64, status, total string) int64 {
	t.Helper()
	var id int64
	if status == "canceled" {
		if err := db.QueryRowContext(ctx, `INSERT INTO invoices
            (client_id, billing_period_id, status, account_balance_before_issue, products_total,
             account_balance_after_charge, canceled_at, cancellation_reason)
            VALUES ($1, $2, 'canceled', 0, $3, $3, CURRENT_TIMESTAMP, 'fixture') RETURNING id`, clientID, periodID, total).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO invoices
        (client_id, billing_period_id, status, account_balance_before_issue, products_total, account_balance_after_charge)
        VALUES ($1, $2, 'issued', 0, $3, $3) RETURNING id`, clientID, periodID, total).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
