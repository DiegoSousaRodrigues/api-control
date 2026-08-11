package accountv2

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newStoreMock(t *testing.T) (*Store, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 sqlDB,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(database)
	if err != nil {
		t.Fatal(err)
	}
	return store, mock
}

func TestWithLockedClientCommitsAfterRowLock(t *testing.T) {
	store, mock := newStoreMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT "id" FROM "clients" WHERE "clients"\."id" = \$1 LIMIT \$2 FOR UPDATE`).
		WithArgs(int64(42), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	mock.ExpectCommit()

	called := false
	err := store.WithLockedClient(context.Background(), 42, func(AccountRepository) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("transaction callback was not called")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestWithLockedClientRollsBackCallbackFailure(t *testing.T) {
	store, mock := newStoreMock(t)
	want := errors.New("allocation failed")
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT "id" FROM "clients" WHERE "clients"\."id" = \$1 LIMIT \$2 FOR UPDATE`).
		WithArgs(int64(42), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	mock.ExpectRollback()

	err := store.WithLockedClient(context.Background(), 42, func(AccountRepository) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFIFOQueriesAndBalanceFormulaRemainAuthoritative(t *testing.T) {
	checks := []struct {
		name string
		sql  string
		want []string
	}{
		{
			name: "invoice FIFO",
			sql:  openInvoicesSQL,
			want: []string{"i.status = 'issued'", "pa.status = 'active'", "ORDER BY bp.period_start ASC, i.created_at ASC, i.id ASC"},
		},
		{
			name: "payment FIFO",
			sql:  availablePaymentsSQL,
			want: []string{"p.status = 'posted'", "pa.status = 'active'", "ORDER BY p.effective_date ASC, p.created_at ASC, p.id ASC"},
		},
		{
			name: "account position excludes allocations",
			sql:  accountPositionSQL,
			want: []string{"SUM(products_total)", "status = 'issued'", "SUM(amount)", "status = 'posted'"},
		},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			for _, fragment := range check.want {
				if !strings.Contains(check.sql, fragment) {
					t.Errorf("query does not contain %q", fragment)
				}
			}
			if check.sql == accountPositionSQL && strings.Contains(strings.ToLower(check.sql), "payment_allocations") {
				t.Fatal("allocations must not affect net account position")
			}
		})
	}
}

func TestValidateConservationBindsClientToEverySubquery(t *testing.T) {
	store, mock := newStoreMock(t)
	repository := &transactionRepository{db: store.db}
	mock.ExpectQuery(`SELECT\s+\(\s+SELECT COUNT`).
		WithArgs(int64(7), int64(7), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"invoice_overallocated", "payment_overallocated", "invalid_active_allocation",
		}).AddRow(0, 0, 0))

	if err := repository.ValidateConservation(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPaymentCreditReturnsPostedPaymentWithZeroUnallocatedAmount(t *testing.T) {
	store, mock := newStoreMock(t)
	repository := &transactionRepository{db: store.db}
	createdAt := time.Date(2026, time.August, 11, 12, 0, 0, 0, time.UTC)
	effectiveDate := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`SELECT\s+p\.id,`).
		WithArgs(int64(7), int64(21)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "client_id", "effective_date", "created_at", "amount", "allocated_amount", "unallocated_amount",
		}).AddRow(21, 7, effectiveDate, createdAt, "100.00", "100.00", "0.00"))

	payment, err := repository.PaymentCredit(context.Background(), 7, 21)
	if err != nil {
		t.Fatal(err)
	}
	if payment.ID != 21 || !payment.Amount.Equal(decimal.NewFromInt(100)) || !payment.UnallocatedAmount.IsZero() {
		t.Fatalf("payment credit = %#v", payment)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
