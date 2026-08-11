package paymentv2_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/api-control/internal/accountv2"
	"github.com/api-control/internal/invoicev2"
	"github.com/api-control/internal/paymentv2"
	schemav2 "github.com/api-control/internal/schema_v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var safeSchema = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// TestPostgresCrossServiceScenario is opt-in and creates only a unique schema
// in a database explicitly authorized as disposable.
func TestPostgresCrossServiceScenario(t *testing.T) {
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
	schema := fmt.Sprintf("control_payment_v2_%d", time.Now().UnixNano())
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
	clientID, productID := insertFixture(t, ctx, testDB)
	database, err := gorm.Open(postgres.New(postgres.Config{Conn: testDB, PreferSimpleProtocol: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, time.November, 20, 15, 0, 0, 0, time.UTC) }
	invoiceStore, _ := invoicev2.NewStore(database)
	invoiceService, _ := invoicev2.NewService(invoiceStore, invoiceStore, now)
	paymentStore, _ := paymentv2.NewStore(database)
	paymentService, _ := paymentv2.NewService(paymentStore, paymentStore, now)

	issue500(t, ctx, invoiceService, clientID, productID, 9)
	post(t, ctx, paymentService, clientID, "250", "2026-09-15")
	issue500(t, ctx, invoiceService, clientID, productID, 10)
	credit100 := post(t, ctx, paymentService, clientID, "850", "2026-10-15")
	if !credit100.Account.CreditAmount.Decimal().Equal(decimal.NewFromInt(100)) {
		t.Fatalf("credit after 850 = %#v", credit100.Account)
	}
	credit300 := post(t, ctx, paymentService, clientID, "200", "2026-10-20")
	if !credit300.Account.CreditAmount.Decimal().Equal(decimal.NewFromInt(300)) {
		t.Fatalf("credit after 200 = %#v", credit300.Account)
	}

	accountStore, _ := accountv2.NewStore(database)
	readService, _ := accountv2.NewReadService(accountStore, now)
	summary, err := readService.Summary(ctx, clientID)
	if err != nil || summary.Position != accountv2.PositionCredit || !summary.CreditAmount.Decimal().Equal(decimal.NewFromInt(300)) {
		t.Fatalf("summary=%#v err=%v", summary, err)
	}
	statement, err := readService.Statement(ctx, clientID, accountv2.StatementFilter{Limit: 20})
	if err != nil || len(statement.Items) != 5 {
		t.Fatalf("statement events=%d err=%v", len(statement.Items), err)
	}
}

func insertFixture(t *testing.T, ctx context.Context, db *sql.DB) (int64, int64) {
	t.Helper()
	var clientID, productID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO clients
        (name, document, phone, street, neighborhood, address_number, address_type)
        VALUES ('Cross service', '770001', '1', 'Street', 'Center', '1', 'home') RETURNING id`).Scan(&clientID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO products (name, purchase_price, sale_price)
        VALUES ('Product', 300.00, 500.00) RETURNING id`).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	return clientID, productID
}

func issue500(t *testing.T, ctx context.Context, service *invoicev2.Service, clientID, productID int64, month int) {
	t.Helper()
	if _, err := service.Issue(ctx, invoicev2.IssueRequest{ClientID: clientID, Year: 2026, Month: month,
		Products: []invoicev2.IssueProduct{{ProductID: productID, Quantity: 1}}}); err != nil {
		t.Fatal(err)
	}
}

func post(t *testing.T, ctx context.Context, service *paymentv2.Service, clientID int64, value, effectiveDate string) *paymentv2.MutationResponse {
	t.Helper()
	amount := accountv2.NewJSONAmount(decimal.RequireFromString(value))
	response, err := service.Create(ctx, paymentv2.CreateRequest{ClientID: clientID, Amount: &amount, EffectiveDate: effectiveDate})
	if err != nil {
		t.Fatal(err)
	}
	return response
}
