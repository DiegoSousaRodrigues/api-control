package repository

import (
	"context"

	"github.com/api-control/internal/domain"
	"github.com/shopspring/decimal"
)

var ReportRepository IReportRepository = &reportRepository{}

type IReportRepository interface {
	ClientBalance(context.Context, int64) (ReportClientProjection, []ClientBalanceMonthProjection, error)
}

type ReportClientProjection struct {
	ID     int64
	Name   string
	Active bool
}

type ClientBalanceMonthProjection struct {
	OrderYear            *int16
	OrderMonth           *int16
	OrderCount           int64
	QuantityTotal        int64
	PurchaseTotal        *decimal.Decimal
	SaleTotal            decimal.Decimal
	ProfitTotal          *decimal.Decimal
	MissingCostItemCount int64
}

type reportRepository struct {
	db domain.BaseRepository
}

func (repository *reportRepository) ClientBalance(ctx context.Context, clientID int64) (ReportClientProjection, []ClientBalanceMonthProjection, error) {
	db := repository.db.PSQL().WithContext(ctx)
	var client ReportClientProjection
	if err := db.Model(&domain.Client{}).
		Select("id", "name", "active").
		Where("id = ?", clientID).
		Take(&client).Error; err != nil {
		return ReportClientProjection{}, nil, err
	}

	months := make([]ClientBalanceMonthProjection, 0)
	if err := db.Raw(clientBalanceByMonthSQL, clientID).Scan(&months).Error; err != nil {
		return ReportClientProjection{}, nil, err
	}
	return client, months, nil
}

const clientBalanceByMonthSQL = `
SELECT
    o.order_year,
    o.order_month,
    COUNT(DISTINCT o.id)::bigint AS order_count,
    COALESCE(SUM(os.quantity), 0)::bigint AS quantity_total,
    COALESCE(SUM(os.price), 0) AS sale_total,
    COUNT(os.id) FILTER (
        WHERE os.snapshot_version <> 1
           OR os.purchase_total IS NULL
           OR os.unit_purchase_price IS NULL
           OR os.unit_sale_price IS NULL
    )::bigint AS missing_cost_item_count,
    CASE
        WHEN COUNT(os.id) FILTER (
            WHERE os.snapshot_version <> 1
               OR os.purchase_total IS NULL
               OR os.unit_purchase_price IS NULL
               OR os.unit_sale_price IS NULL
        ) = 0
        THEN COALESCE(SUM(os.purchase_total), 0)
        ELSE NULL
    END AS purchase_total,
    CASE
        WHEN COUNT(os.id) FILTER (
            WHERE os.snapshot_version <> 1
               OR os.purchase_total IS NULL
               OR os.unit_purchase_price IS NULL
               OR os.unit_sale_price IS NULL
        ) = 0
        THEN COALESCE(SUM(os.price - os.purchase_total), 0)
        ELSE NULL
    END AS profit_total
FROM "order" o
JOIN order_sku os ON os.order_id = o.id
WHERE o.client_id = ?
GROUP BY o.order_year, o.order_month
ORDER BY o.order_year DESC NULLS LAST, o.order_month DESC NULLS LAST`
