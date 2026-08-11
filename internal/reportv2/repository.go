package reportv2

import (
	"context"
	"errors"
	"time"

	domainv2 "github.com/api-control/internal/domain/v2"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type ClientProjection struct {
	ID     int64
	Name   string
	Active bool
}

type MonthProjection struct {
	PeriodStart            time.Time
	InvoiceCount           int64
	QuantityTotal          int64
	PurchaseTotal          decimal.Decimal
	SaleTotal              decimal.Decimal
	ProfitTotal            decimal.Decimal
	MismatchedInvoiceCount int64
}

type Repository interface {
	ClientBalance(context.Context, int64) (ClientProjection, []MonthProjection, error)
}

type Store struct{ db *gorm.DB }

func NewStore(db *gorm.DB) (*Store, error) {
	if db == nil {
		return nil, errors.New("report v2 database is required")
	}
	return &Store{db: db}, nil
}

func (store *Store) ClientBalance(ctx context.Context, clientID int64) (ClientProjection, []MonthProjection, error) {
	var client ClientProjection
	if err := store.db.WithContext(ctx).Model(&domainv2.Client{}).
		Select("id", "name", "active").Where("id = ?", clientID).Take(&client).Error; err != nil {
		return ClientProjection{}, nil, err
	}
	months := make([]MonthProjection, 0)
	if err := store.db.WithContext(ctx).Raw(clientBalanceSQL, clientID).Scan(&months).Error; err != nil {
		return ClientProjection{}, nil, err
	}
	return client, months, nil
}

const clientBalanceSQL = `
WITH invoice_totals AS (
    SELECT
        i.id,
        i.client_id,
        bp.period_start,
        i.products_total,
        COUNT(ii.id)::bigint AS item_count,
        COALESCE(SUM(ii.quantity), 0)::bigint AS quantity_total,
        COALESCE(SUM(ii.purchase_total), 0) AS purchase_total,
        COALESCE(SUM(ii.sale_total), 0) AS sale_total,
        COALESCE(SUM(ii.profit_total), 0) AS profit_total
    FROM invoices i
    JOIN billing_periods bp ON bp.id = i.billing_period_id
    LEFT JOIN invoice_items ii ON ii.invoice_id = i.id
    WHERE i.client_id = ? AND i.status = 'issued'
    GROUP BY i.id, i.client_id, bp.period_start, i.products_total
)
SELECT
    period_start,
    COUNT(*)::bigint AS invoice_count,
    COALESCE(SUM(quantity_total), 0)::bigint AS quantity_total,
    COALESCE(SUM(purchase_total), 0) AS purchase_total,
    COALESCE(SUM(sale_total), 0) AS sale_total,
    COALESCE(SUM(profit_total), 0) AS profit_total,
    COUNT(*) FILTER (WHERE item_count = 0 OR sale_total <> products_total)::bigint AS mismatched_invoice_count
FROM invoice_totals
GROUP BY period_start
ORDER BY period_start DESC`
