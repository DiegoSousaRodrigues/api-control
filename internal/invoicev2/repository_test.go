package invoicev2

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/api-control/internal/accountv2"
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
	database, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, PreferSimpleProtocol: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(database)
	if err != nil {
		t.Fatal(err)
	}
	return store, mock
}

func TestWithLockedClientSharesOneTransactionWithAccountRepository(t *testing.T) {
	store, mock := newStoreMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT "id" FROM "clients" WHERE "clients"\."id" = \$1 LIMIT \$2 FOR UPDATE`).
		WithArgs(int64(42), 1).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	mock.ExpectQuery(`SELECT\s+COALESCE`).WithArgs(int64(42), int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"net_balance"}).AddRow("-25.00"))
	mock.ExpectCommit()

	err := store.WithLockedClient(context.Background(), 42, func(_ Repository, accountRepository accountv2.AccountRepository) error {
		position, err := accountRepository.Position(context.Background(), 42)
		if err != nil {
			return err
		}
		if position.State != accountv2.PositionCredit {
			t.Fatalf("position = %#v", position)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestWithLockedClientRollsBackInvoiceFailure(t *testing.T) {
	store, mock := newStoreMock(t)
	want := errors.New("snapshot insert failed")
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT "id" FROM "clients" WHERE "clients"\."id" = \$1 LIMIT \$2 FOR UPDATE`).
		WithArgs(int64(42), 1).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	mock.ExpectRollback()

	err := store.WithLockedClient(context.Background(), 42, func(Repository, accountv2.AccountRepository) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectionDoesNotSelectClientPersonalData(t *testing.T) {
	for _, forbidden := range []string{"document", "phone", "street", "birth_date", "postal_code"} {
		if containsSQLWord(invoiceProjectionSQL, forbidden) {
			t.Fatalf("invoice projection leaks client field %q", forbidden)
		}
	}
}

func containsSQLWord(query, word string) bool {
	for index := 0; index+len(word) <= len(query); index++ {
		if query[index:index+len(word)] == word {
			return true
		}
	}
	return false
}
