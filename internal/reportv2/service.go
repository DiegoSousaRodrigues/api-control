package reportv2

import (
	"context"
	"errors"

	"github.com/shopspring/decimal"
)

var (
	ErrInvalidClientID        = errors.New("client id must be positive")
	ErrInconsistentSnapshots  = errors.New("invoice item sale snapshots do not reconcile with invoice total")
	ErrInconsistentAggregate  = errors.New("client balance aggregate is inconsistent")
	ErrReportAmountOutOfRange = errors.New("client balance total is outside supported range")
)

type Service struct{ repository Repository }

func NewService(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, errors.New("report v2 repository is required")
	}
	return &Service{repository: repository}, nil
}

func (service *Service) ClientBalance(ctx context.Context, clientID int64) (*ClientBalanceResponse, error) {
	if clientID <= 0 {
		return nil, ErrInvalidClientID
	}
	client, projections, err := service.repository.ClientBalance(ctx, clientID)
	if err != nil {
		return nil, err
	}
	response := &ClientBalanceResponse{
		Client: ClientResponse{ID: client.ID, Name: client.Name, Active: client.Active},
		Months: make([]MonthResponse, 0, len(projections)),
	}
	totalPurchase, totalSale, totalProfit := decimal.Zero, decimal.Zero, decimal.Zero
	for _, projection := range projections {
		if projection.PeriodStart.IsZero() || projection.MismatchedInvoiceCount != 0 {
			return nil, ErrInconsistentSnapshots
		}
		if !projection.ProfitTotal.Equal(projection.SaleTotal.Sub(projection.PurchaseTotal)) {
			return nil, ErrInconsistentAggregate
		}
		if !validReportAmount(projection.PurchaseTotal) || !validReportAmount(projection.SaleTotal) || !validReportAmount(projection.ProfitTotal) {
			return nil, ErrReportAmountOutOfRange
		}
		month := MonthResponse{Year: projection.PeriodStart.Year(), Month: int(projection.PeriodStart.Month()),
			TotalsResponse: TotalsResponse{InvoiceCount: projection.InvoiceCount, QuantityTotal: projection.QuantityTotal,
				PurchaseTotal: newAmount(projection.PurchaseTotal), SaleTotal: newAmount(projection.SaleTotal),
				ProfitTotal: newAmount(projection.ProfitTotal)}}
		response.Months = append(response.Months, month)
		response.Totals.InvoiceCount += projection.InvoiceCount
		response.Totals.QuantityTotal += projection.QuantityTotal
		totalPurchase = totalPurchase.Add(projection.PurchaseTotal)
		totalSale = totalSale.Add(projection.SaleTotal)
		totalProfit = totalProfit.Add(projection.ProfitTotal)
	}
	if !totalProfit.Equal(totalSale.Sub(totalPurchase)) {
		return nil, ErrInconsistentAggregate
	}
	if !validReportAmount(totalPurchase) || !validReportAmount(totalSale) || !validReportAmount(totalProfit) {
		return nil, ErrReportAmountOutOfRange
	}
	response.Totals.PurchaseTotal = newAmount(totalPurchase)
	response.Totals.SaleTotal = newAmount(totalSale)
	response.Totals.ProfitTotal = newAmount(totalProfit)
	return response, nil
}

func validReportAmount(value decimal.Decimal) bool {
	return value.Abs().LessThanOrEqual(maxReportAmount) && value.Exponent() >= -2
}
