package service

import (
	"context"
	"errors"
	"testing"

	"github.com/api-control/internal/repository"
	"github.com/shopspring/decimal"
)

type reportRepositoryFake struct {
	client repository.ReportClientProjection
	months []repository.ClientBalanceMonthProjection
	err    error
}

func (fake *reportRepositoryFake) ClientBalance(context.Context, int64) (repository.ReportClientProjection, []repository.ClientBalanceMonthProjection, error) {
	return fake.client, fake.months, fake.err
}

func TestClientBalanceAggregatesCompleteAndNegativeProfitMonths(t *testing.T) {
	original := repository.ReportRepository
	purchaseJanuary := decimal.RequireFromString("70.00")
	profitJanuary := decimal.RequireFromString("30.00")
	purchaseFebruary := decimal.RequireFromString("60.00")
	profitFebruary := decimal.RequireFromString("-10.00")
	year, january, february := int16(2026), int16(1), int16(2)
	repository.ReportRepository = &reportRepositoryFake{
		client: repository.ReportClientProjection{ID: 7, Name: "Cliente", Active: false},
		months: []repository.ClientBalanceMonthProjection{
			{OrderYear: &year, OrderMonth: &february, OrderCount: 1, QuantityTotal: 2, PurchaseTotal: &purchaseFebruary, SaleTotal: decimal.RequireFromString("50.00"), ProfitTotal: &profitFebruary},
			{OrderYear: &year, OrderMonth: &january, OrderCount: 2, QuantityTotal: 4, PurchaseTotal: &purchaseJanuary, SaleTotal: decimal.RequireFromString("100.00"), ProfitTotal: &profitJanuary},
		},
	}
	t.Cleanup(func() { repository.ReportRepository = original })

	report, err := ReportService.ClientBalance(context.Background(), 7)
	if err != nil {
		t.Fatalf("ClientBalance() error = %v", err)
	}
	if report.Client.Active || report.Client.ID != 7 {
		t.Fatalf("client = %+v, want inactive client 7", report.Client)
	}
	if report.Totals.OrderCount != 3 || report.Totals.QuantityTotal != 6 {
		t.Fatalf("totals counts = %+v", report.Totals)
	}
	if got := report.Totals.SaleTotal.Decimal(); !got.Equal(decimal.RequireFromString("150.00")) {
		t.Fatalf("sale total = %s, want 150.00", got)
	}
	if report.Totals.PurchaseTotal == nil || !report.Totals.PurchaseTotal.Decimal().Equal(decimal.RequireFromString("130.00")) {
		t.Fatalf("purchase total = %v, want 130.00", report.Totals.PurchaseTotal)
	}
	if report.Totals.ProfitTotal == nil || !report.Totals.ProfitTotal.Decimal().Equal(decimal.RequireFromString("20.00")) {
		t.Fatalf("profit total = %v, want 20.00", report.Totals.ProfitTotal)
	}
	if !report.Totals.CostComplete || len(report.Months) != 2 {
		t.Fatalf("report completeness/months = %+v / %d", report.Totals, len(report.Months))
	}
}

func TestClientBalanceMarksLegacyCostAsUnavailable(t *testing.T) {
	original := repository.ReportRepository
	repository.ReportRepository = &reportRepositoryFake{
		client: repository.ReportClientProjection{ID: 8, Name: "Legado", Active: true},
		months: []repository.ClientBalanceMonthProjection{{
			OrderCount: 1, QuantityTotal: 3, SaleTotal: decimal.RequireFromString("45.00"), MissingCostItemCount: 1,
		}},
	}
	t.Cleanup(func() { repository.ReportRepository = original })

	report, err := ReportService.ClientBalance(context.Background(), 8)
	if err != nil {
		t.Fatalf("ClientBalance() error = %v", err)
	}
	if report.Totals.CostComplete || report.Totals.PurchaseTotal != nil || report.Totals.ProfitTotal != nil {
		t.Fatalf("legacy totals must not invent cost/profit: %+v", report.Totals)
	}
	if report.Months[0].CostComplete || report.Months[0].PurchaseTotal != nil || report.Months[0].ProfitTotal != nil {
		t.Fatalf("legacy month must not invent cost/profit: %+v", report.Months[0])
	}
}

func TestClientBalanceRejectsInconsistentRepositoryAggregate(t *testing.T) {
	original := repository.ReportRepository
	repository.ReportRepository = &reportRepositoryFake{
		client: repository.ReportClientProjection{ID: 9},
		months: []repository.ClientBalanceMonthProjection{{SaleTotal: decimal.Zero}},
	}
	t.Cleanup(func() { repository.ReportRepository = original })

	_, err := ReportService.ClientBalance(context.Background(), 9)
	if !errors.Is(err, ErrReportIncompleteAggregate) {
		t.Fatalf("error = %v, want ErrReportIncompleteAggregate", err)
	}
}

func TestClientBalanceReturnsEmptyMonthsAsArray(t *testing.T) {
	original := repository.ReportRepository
	repository.ReportRepository = &reportRepositoryFake{client: repository.ReportClientProjection{ID: 10, Name: "Sem pedidos", Active: true}}
	t.Cleanup(func() { repository.ReportRepository = original })

	report, err := ReportService.ClientBalance(context.Background(), 10)
	if err != nil {
		t.Fatalf("ClientBalance() error = %v", err)
	}
	if report.Months == nil || len(report.Months) != 0 {
		t.Fatalf("months = %#v, want non-nil empty slice", report.Months)
	}
}
