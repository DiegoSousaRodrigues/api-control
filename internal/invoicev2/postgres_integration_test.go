package invoicev2

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	schemav2 "github.com/api-control/internal/schema_v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var safeSchemaName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// TestPostgresInvoiceLifecycle is opt-in and only operates in a uniquely named
// schema when the caller explicitly authorizes the configured DB as disposable.
func TestPostgresInvoiceLifecycle(t *testing.T) {
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
	adminDB := stdlib.OpenDB(*config)
	defer adminDB.Close()
	schema := fmt.Sprintf("control_invoice_v2_%d", time.Now().UnixNano())
	if !safeSchemaName.MatchString(schema) {
		t.Fatalf("unsafe generated schema %q", schema)
	}
	if _, err := adminDB.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, err := adminDB.ExecContext(cleanupCtx, `DROP SCHEMA `+schema+` CASCADE`); err != nil {
			t.Errorf("drop isolated schema: %v", err)
		}
	})
	testConfig := config.Copy()
	testConfig.RuntimeParams["search_path"] = schema
	testDB := stdlib.OpenDB(*testConfig)
	defer testDB.Close()
	if err := schemav2.ApplyUp(ctx, testDB); err != nil {
		t.Fatalf("apply staging baseline: %v", err)
	}

	clientID, productID := insertInvoiceFixture(t, ctx, testDB)
	database, err := gorm.Open(postgres.New(postgres.Config{Conn: testDB, PreferSimpleProtocol: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	store, _ := NewStore(database)
	service, err := NewService(store, store, func() time.Time {
		return time.Date(2026, time.November, 15, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}
	issued, err := service.Issue(ctx, IssueRequest{ClientID: clientID, Year: 2026, Month: 10,
		Products: []IssueProduct{{ProductID: productID, Quantity: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if !issued.Invoice.OpenAmount.Decimal().Equal(decimal.NewFromInt(200)) ||
		!issued.Invoice.Items[0].ProfitTotal.Decimal().Equal(decimal.NewFromInt(-100)) {
		t.Fatalf("issued invoice = %#v", issued.Invoice)
	}
	canceled, err := service.Cancel(ctx, issued.Invoice.ID, CancelRequest{Reason: "fixture cancellation"})
	if err != nil {
		t.Fatal(err)
	}
	if canceled.Account.Position != "credit" || !canceled.Account.CreditAmount.Decimal().Equal(decimal.NewFromInt(300)) {
		t.Fatalf("position after cancellation = %#v", canceled.Account)
	}
}

func insertInvoiceFixture(t *testing.T, ctx context.Context, db *sql.DB) (int64, int64) {
	t.Helper()
	var clientID, productID int64
	if err := db.QueryRowContext(ctx, `
INSERT INTO clients (name, document, phone, street, neighborhood, address_number, address_type)
VALUES ('Invoice fixture', '880001', '1', 'Street', 'Center', '1', 'home') RETURNING id`).Scan(&clientID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `
INSERT INTO products (name, purchase_price, sale_price)
VALUES ('Negative profit fixture', 600.00, 500.00) RETURNING id`).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO payments (client_id, amount, effective_date, status)
VALUES ($1, 300.00, '2026-09-15', 'posted')`, clientID); err != nil {
		t.Fatal(err)
	}
	return clientID, productID
}
