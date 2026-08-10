package service

import (
	"context"
	"errors"

	"github.com/api-control/internal/dto"
	"github.com/api-control/internal/repository"
	"github.com/shopspring/decimal"
)

var ReportService IReportService = &reportService{}

var ErrReportIncompleteAggregate = errors.New("report aggregate is inconsistent")

type IReportService interface {
	ClientBalance(context.Context, int64) (*dto.ClientBalanceReportDTO, error)
}

type reportService struct{}

func (service *reportService) ClientBalance(ctx context.Context, clientID int64) (*dto.ClientBalanceReportDTO, error) {
	client, projections, err := repository.ReportRepository.ClientBalance(ctx, clientID)
	if err != nil {
		return nil, err
	}

	months := make([]dto.ClientBalanceMonthDTO, 0, len(projections))
	totalOrders := int64(0)
	totalQuantity := int64(0)
	totalMissing := int64(0)
	totalSale := decimal.Zero
	totalPurchase := decimal.Zero
	totalProfit := decimal.Zero

	for _, projection := range projections {
		complete := projection.MissingCostItemCount == 0
		if complete && (projection.PurchaseTotal == nil || projection.ProfitTotal == nil) {
			return nil, ErrReportIncompleteAggregate
		}
		month := dto.ClientBalanceMonthDTO{
			Year:  projection.OrderYear,
			Month: projection.OrderMonth,
			ReportBalanceTotalsDTO: dto.ReportBalanceTotalsDTO{
				OrderCount:           projection.OrderCount,
				QuantityTotal:        projection.QuantityTotal,
				PurchaseTotal:        reportMoneyPointer(projection.PurchaseTotal),
				SaleTotal:            dto.NewMoney(projection.SaleTotal),
				ProfitTotal:          reportMoneyPointer(projection.ProfitTotal),
				CostComplete:         complete,
				MissingCostItemCount: projection.MissingCostItemCount,
			},
		}
		months = append(months, month)
		totalOrders += projection.OrderCount
		totalQuantity += projection.QuantityTotal
		totalMissing += projection.MissingCostItemCount
		totalSale = totalSale.Add(projection.SaleTotal)
		if complete {
			totalPurchase = totalPurchase.Add(*projection.PurchaseTotal)
			totalProfit = totalProfit.Add(*projection.ProfitTotal)
		}
	}

	totalsComplete := totalMissing == 0
	var purchaseTotal *dto.Money
	var profitTotal *dto.Money
	if totalsComplete {
		purchaseTotal = reportMoneyPointer(&totalPurchase)
		profitTotal = reportMoneyPointer(&totalProfit)
	}

	return &dto.ClientBalanceReportDTO{
		Client: dto.ReportClientDTO{ID: client.ID, Name: client.Name, Active: client.Active},
		Totals: dto.ReportBalanceTotalsDTO{
			OrderCount:           totalOrders,
			QuantityTotal:        totalQuantity,
			PurchaseTotal:        purchaseTotal,
			SaleTotal:            dto.NewMoney(totalSale),
			ProfitTotal:          profitTotal,
			CostComplete:         totalsComplete,
			MissingCostItemCount: totalMissing,
		},
		Months: months,
	}, nil
}

func reportMoneyPointer(value *decimal.Decimal) *dto.Money {
	if value == nil {
		return nil
	}
	money := dto.NewMoney(value.Copy())
	return &money
}
