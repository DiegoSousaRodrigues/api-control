package paymentv2

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

func TestUnitOfWorkLocksClientAndRollsBack(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	database, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, PreferSimpleProtocol: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	store, _ := NewStore(database)
	want := errors.New("write failed")
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT "id" FROM "clients" WHERE "clients"\."id" = \$1 LIMIT \$2 FOR UPDATE`).
		WithArgs(int64(7), 1).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	mock.ExpectRollback()
	err = store.WithLockedClient(context.Background(), 7, func(Repository, accountv2.AccountRepository) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
