package stagingv2

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	domainv2 "github.com/api-control/internal/domain/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newRepositoryMock(t *testing.T) (*Repository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 sqlDB,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	repository, err := NewRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	return repository, mock
}

func TestRepositoryUsesPluralUsersTableAndNormalizedLogin(t *testing.T) {
	repository, mock := newRepositoryMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "users" WHERE login = $1`)).
		WithArgs("admin@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	exists, err := repository.LoginExists(context.Background(), " ADMIN@Example.com ")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected normalized login to exist")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryUpdateClientChecksRowsAffected(t *testing.T) {
	repository, mock := newRepositoryMock(t)
	mock.ExpectExec(`UPDATE "clients" SET`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repository.UpdateClient(context.Background(), 10, domainv2.Client{
		Name: "Client", Document: "123.456.789-00", Phone: "1", Street: "Street",
		Neighborhood: "Center", AddressNumber: "10", AddressType: "home",
	})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("error = %v, want record not found", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryRejectsInvalidIdentifiersWithoutQuery(t *testing.T) {
	repository, mock := newRepositoryMock(t)
	if _, err := repository.FindProduct(context.Background(), 0); !errors.Is(err, ErrInvalidIdentifier) {
		t.Fatalf("FindProduct error = %v", err)
	}
	if err := repository.SetProductActive(context.Background(), -1, true); !errors.Is(err, ErrInvalidIdentifier) {
		t.Fatalf("SetProductActive error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
