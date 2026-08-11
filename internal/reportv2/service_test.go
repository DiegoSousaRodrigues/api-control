package reportv2

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

type repositoryStub struct {
	client ClientProjection
	months []MonthProjection
	err    error
}

func (stub repositoryStub) ClientBalance(context.Context, int64) (ClientProjection, []MonthProjection, error) {
	return stub.client, stub.months, stub.err
}

func TestClientBalanceAggregatesSnapshotsAndNegativeProfit(t *testing.T) {
	repository := repositoryStub{client: ClientProjection{ID: 7, Name: "Client", Active: false}, months: []MonthProjection{
		{PeriodStart: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), InvoiceCount: 2, QuantityTotal: 4,
			PurchaseTotal: decimal.NewFromInt(120), SaleTotal: decimal.NewFromInt(100), ProfitTotal: decimal.NewFromInt(-20)},
		{PeriodStart: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), InvoiceCount: 1, QuantityTotal: 2,
			PurchaseTotal: decimal.NewFromInt(30), SaleTotal: decimal.NewFromInt(50), ProfitTotal: decimal.NewFromInt(20)},
	}}
	service, _ := NewService(repository)
	response, err := service.ClientBalance(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if response.Client.Active || response.Totals.InvoiceCount != 3 || response.Totals.QuantityTotal != 6 || len(response.Months) != 2 {
		t.Fatalf("response = %#v", response)
	}
	if !response.Totals.PurchaseTotal.Decimal().Equal(decimal.NewFromInt(150)) ||
		!response.Totals.SaleTotal.Decimal().Equal(decimal.NewFromInt(150)) || !response.Totals.ProfitTotal.Decimal().IsZero() {
		t.Fatalf("totals = %#v", response.Totals)
	}
	if response.Months[0].Year != 2026 || response.Months[0].Month != 8 ||
		!response.Months[0].ProfitTotal.Decimal().Equal(decimal.NewFromInt(-20)) {
		t.Fatalf("month = %#v", response.Months[0])
	}
}

func TestClientBalanceEmptyClientReturnsZerosAndArray(t *testing.T) {
	service, _ := NewService(repositoryStub{client: ClientProjection{ID: 8, Name: "Empty", Active: true}})
	response, err := service.ClientBalance(context.Background(), 8)
	if err != nil {
		t.Fatal(err)
	}
	if response.Months == nil || len(response.Months) != 0 || response.Totals.InvoiceCount != 0 ||
		!response.Totals.PurchaseTotal.Decimal().IsZero() || !response.Totals.SaleTotal.Decimal().IsZero() {
		t.Fatalf("empty response = %#v", response)
	}
}

func TestClientBalanceRejectsInvoiceItemMismatch(t *testing.T) {
	service, _ := NewService(repositoryStub{client: ClientProjection{ID: 9}, months: []MonthProjection{{
		PeriodStart: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), MismatchedInvoiceCount: 1,
	}}})
	_, err := service.ClientBalance(context.Background(), 9)
	if !errors.Is(err, ErrInconsistentSnapshots) {
		t.Fatalf("error = %v", err)
	}
}

func TestClientBalanceRejectsInvalidProfitProjection(t *testing.T) {
	service, _ := NewService(repositoryStub{client: ClientProjection{ID: 9}, months: []MonthProjection{{
		PeriodStart: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), PurchaseTotal: decimal.NewFromInt(20),
		SaleTotal: decimal.NewFromInt(30), ProfitTotal: decimal.NewFromInt(11),
	}}})
	_, err := service.ClientBalance(context.Background(), 9)
	if !errors.Is(err, ErrInconsistentAggregate) {
		t.Fatalf("error = %v", err)
	}
}
