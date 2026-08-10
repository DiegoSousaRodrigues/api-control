package repository

import (
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/api-control/internal/domain"
	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type orderDatabaseStub struct {
	db *gorm.DB
}

func (stub orderDatabaseStub) PSQL() *gorm.DB { return stub.db }

func TestApplyOrderSkuSnapshotAcceptsZeroCostAndNegativeProfit(t *testing.T) {
	zero := decimal.Zero
	item := domain.OrderSku{Quantity: 2}
	sku := domain.Sku{
		Name:          "Promotional product",
		PurchasePrice: &zero,
		SalePrice:     decimal.RequireFromString("0.01"),
	}

	if err := applyOrderSkuSnapshot(&item, sku); err != nil {
		t.Fatal(err)
	}
	if item.PurchaseTotal == nil || !item.PurchaseTotal.IsZero() {
		t.Fatalf("PurchaseTotal = %v, want zero", item.PurchaseTotal)
	}

	highCost := decimal.RequireFromString("20.00")
	item = domain.OrderSku{Quantity: 2}
	sku.PurchasePrice = &highCost
	sku.SalePrice = decimal.RequireFromString("10.00")
	if err := applyOrderSkuSnapshot(&item, sku); err != nil {
		t.Fatalf("negative profit must be allowed: %v", err)
	}
	profit := item.Price.Sub(*item.PurchaseTotal)
	if !profit.Equal(decimal.RequireFromString("-20.00")) {
		t.Fatalf("profit = %s, want -20.00", profit)
	}
}

func TestApplyOrderSkuSnapshotRejectsMissingCostWithoutMutation(t *testing.T) {
	item := domain.OrderSku{Quantity: 2, Name: "request value", Price: decimal.RequireFromString("1.00")}
	err := applyOrderSkuSnapshot(&item, domain.Sku{Name: "database value", SalePrice: decimal.RequireFromString("10.00")})

	if !errors.Is(err, ErrOrderSkuPurchasePriceMissing) {
		t.Fatalf("error = %v, want %v", err, ErrOrderSkuPurchasePriceMissing)
	}
	if item.SnapshotVersion != 0 || item.UnitPurchasePrice != nil || item.PurchaseTotal != nil || item.UnitSalePrice != nil {
		t.Fatalf("item was partially snapshotted: %+v", item)
	}
	if item.Name != "request value" || !item.Price.Equal(decimal.RequireFromString("1.00")) {
		t.Fatalf("item legacy fields were partially mutated: %+v", item)
	}
}

func TestApplyOrderSkuSnapshotRejectsNumericOverflow(t *testing.T) {
	purchase := decimal.RequireFromString("999999999999.99")
	item := domain.OrderSku{Quantity: 2}
	err := applyOrderSkuSnapshot(&item, domain.Sku{PurchasePrice: &purchase, SalePrice: decimal.RequireFromString("1.00")})

	if !errors.Is(err, ErrOrderSkuSnapshotOutOfRange) {
		t.Fatalf("error = %v, want %v", err, ErrOrderSkuSnapshotOutOfRange)
	}
}

func TestOrderAddRollsBackWhenSkuCostIsMissing(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	repository := &orderRepository{db: orderDatabaseStub{db: db}}
	year, month := int16(2026), int16(8)
	order := domain.Order{
		ClientId:   1,
		OrderYear:  &year,
		OrderMonth: &month,
		OrderSkus:  []domain.OrderSku{{SkuID: 7, Quantity: 1}},
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "client" WHERE id = $1 ORDER BY "client"."id" LIMIT $2 FOR UPDATE`)).
		WithArgs(int64(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "active"}).AddRow(1, true))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "sku" WHERE id = $1 ORDER BY "sku"."id" LIMIT $2`)).
		WithArgs(int64(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "purchase_price", "sale_price", "active"}).AddRow(7, "Legacy", nil, "10.00", true))
	mock.ExpectRollback()

	err = repository.Add(order)
	if !errors.Is(err, ErrOrderSkuPurchasePriceMissing) {
		t.Fatalf("error = %v, want %v", err, ErrOrderSkuPurchasePriceMissing)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLedgerChargeUsesHistoricalSaleTotals(t *testing.T) {
	purchaseA := decimal.RequireFromString("4.00")
	purchaseB := decimal.RequireFromString("30.00")
	items := []domain.OrderSku{{Quantity: 3}, {Quantity: 1}}
	if err := applyOrderSkuSnapshot(&items[0], domain.Sku{PurchasePrice: &purchaseA, SalePrice: decimal.RequireFromString("10.00")}); err != nil {
		t.Fatal(err)
	}
	if err := applyOrderSkuSnapshot(&items[1], domain.Sku{PurchasePrice: &purchaseB, SalePrice: decimal.RequireFromString("5.50")}); err != nil {
		t.Fatal(err)
	}

	charge := orderItemsTotal(items)
	if !charge.Equal(decimal.RequireFromString("35.50")) {
		t.Fatalf("charge = %s, want 35.50", charge)
	}
}
