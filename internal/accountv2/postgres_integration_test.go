package accountv2_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/api-control/internal/accountv2"
	schemav2 "github.com/api-control/internal/schema_v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	postgresTestDSNEnvironment   = "V2_SCHEMA_TEST_DATABASE_URL"
	postgresTestGuardEnvironment = "V2_SCHEMA_TEST_ALLOW_DISPOSABLE"
)

var postgresIdentifier = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// TestPostgresClientLockPreventsDoubleSpend is opt-in because it needs a real
// PostgreSQL server. It only creates a uniquely named schema in a database
// explicitly authorized as disposable.
func TestPostgresClientLockPreventsDoubleSpend(t *testing.T) {
	dsn := os.Getenv(postgresTestDSNEnvironment)
	if dsn == "" {
		t.Skip(postgresTestDSNEnvironment + " is not configured")
	}
	if os.Getenv(postgresTestGuardEnvironment) != "yes" {
		t.Fatalf("%s=yes is required for the disposable PostgreSQL test", postgresTestGuardEnvironment)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid disposable PostgreSQL test configuration")
	}
	adminDB := stdlib.OpenDB(*config)
	defer adminDB.Close()
	if err := adminDB.PingContext(ctx); err != nil {
		t.Fatal("cannot connect to disposable PostgreSQL test database")
	}

	schemaName := fmt.Sprintf("control_account_v2_%d", time.Now().UnixNano())
	if !postgresIdentifier.MatchString(schemaName) {
		t.Fatalf("unsafe generated schema name %q", schemaName)
	}
	if _, err := adminDB.ExecContext(ctx, `CREATE SCHEMA `+schemaName); err != nil {
		t.Fatalf("create isolated account test schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, err := adminDB.ExecContext(cleanupCtx, `DROP SCHEMA `+schemaName+` CASCADE`); err != nil {
			t.Errorf("drop isolated account test schema: %v", err)
		}
	})

	testConfig := config.Copy()
	testConfig.RuntimeParams["search_path"] = schemaName
	testDB := stdlib.OpenDB(*testConfig)
	defer testDB.Close()
	if err := testDB.PingContext(ctx); err != nil {
		t.Fatal("cannot enter isolated PostgreSQL account test schema")
	}
	if err := schemav2.ApplyUp(ctx, testDB); err != nil {
		t.Fatalf("apply baseline in isolated account test schema: %v", err)
	}

	clientID, paymentID := insertConcurrentAllocationFixture(t, ctx, testDB)
	store := newPostgresStore(t, testDB)
	service, err := accountv2.NewService(store)
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for range 2 {
		go func() {
			ready.Done()
			<-start
			_, operationErr := service.AllocatePayment(ctx, clientID, paymentID)
			results <- operationErr
		}()
	}
	ready.Wait()
	close(start)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatalf("concurrent allocation failed: %v", err)
		}
	}

	assertConcurrentAllocationTotals(t, ctx, testDB, paymentID)
	position, err := service.Position(ctx, clientID)
	if err != nil {
		t.Fatal(err)
	}
	if position.State != accountv2.PositionDebt || !position.DebtAmount.Equal(decimal.NewFromInt(60)) {
		t.Fatalf("position after concurrent allocation = %#v", position)
	}
}

func newPostgresStore(t *testing.T, db *sql.DB) *accountv2.Store {
	t.Helper()
	database, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	store, err := accountv2.NewStore(database)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func insertConcurrentAllocationFixture(t *testing.T, ctx context.Context, db *sql.DB) (int64, int64) {
	t.Helper()
	var clientID, firstPeriodID, secondPeriodID, paymentID int64
	if err := db.QueryRowContext(ctx, `
		INSERT INTO clients (name, document, phone, street, neighborhood, address_number, address_type)
		VALUES ('Concurrency client', '990001', '1', 'Street', 'Center', '1', 'home') RETURNING id
	`).Scan(&clientID); err != nil {
		t.Fatalf("insert concurrency client: %v", err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO billing_periods (period_start) VALUES ('2026-07-01') RETURNING id`).Scan(&firstPeriodID); err != nil {
		t.Fatalf("insert first concurrency period: %v", err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO billing_periods (period_start) VALUES ('2026-08-01') RETURNING id`).Scan(&secondPeriodID); err != nil {
		t.Fatalf("insert second concurrency period: %v", err)
	}
	for _, periodID := range []int64{firstPeriodID, secondPeriodID} {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO invoices
				(client_id, billing_period_id, status, account_balance_before_issue, products_total, account_balance_after_charge)
			VALUES ($1, $2, 'issued', 0.00, 80.00, 80.00)
		`, clientID, periodID); err != nil {
			t.Fatalf("insert concurrency invoice: %v", err)
		}
	}
	if err := db.QueryRowContext(ctx, `
		INSERT INTO payments (client_id, amount, effective_date, status)
		VALUES ($1, 100.00, '2026-08-10', 'posted') RETURNING id
	`, clientID).Scan(&paymentID); err != nil {
		t.Fatalf("insert concurrency payment: %v", err)
	}
	return clientID, paymentID
}

func assertConcurrentAllocationTotals(t *testing.T, ctx context.Context, db *sql.DB, paymentID int64) {
	t.Helper()
	var allocationCount int
	var allocated decimal.Decimal
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(amount), 0)
		FROM payment_allocations
		WHERE payment_id = $1 AND status = 'active'
	`, paymentID).Scan(&allocationCount, &allocated); err != nil {
		t.Fatalf("read concurrent allocations: %v", err)
	}
	if allocationCount != 2 || !allocated.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("concurrent allocations count=%d total=%s, want count=2 total=100", allocationCount, allocated)
	}

	rows, err := db.QueryContext(ctx, `
		SELECT pa.amount
		FROM payment_allocations pa
		JOIN invoices i ON i.id = pa.invoice_id
		JOIN billing_periods bp ON bp.id = i.billing_period_id
		WHERE pa.payment_id = $1 AND pa.status = 'active'
		ORDER BY bp.period_start ASC, i.created_at ASC, i.id ASC
	`, paymentID)
	if err != nil {
		t.Fatalf("read concurrent allocation FIFO: %v", err)
	}
	defer rows.Close()
	want := []decimal.Decimal{decimal.NewFromInt(80), decimal.NewFromInt(20)}
	var index int
	for rows.Next() {
		var amount decimal.Decimal
		if err := rows.Scan(&amount); err != nil {
			t.Fatalf("scan concurrent allocation FIFO: %v", err)
		}
		if index >= len(want) {
			t.Fatalf("unexpected allocation %d amount=%s", index, amount)
		}
		if !amount.Equal(want[index]) {
			t.Fatalf("allocation %d amount=%s, want FIFO amount=%s", index, amount, want[index])
		}
		index++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate concurrent allocation FIFO: %v", err)
	}
	if index != len(want) {
		t.Fatalf("FIFO allocation rows=%d, want %d", index, len(want))
	}
}
