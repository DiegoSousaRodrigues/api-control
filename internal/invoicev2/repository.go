package invoicev2

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/api-control/internal/accountv2"
	domainv2 "github.com/api-control/internal/domain/v2"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UnitOfWork interface {
	WithLockedClient(context.Context, int64, func(Repository, accountv2.AccountRepository) error) error
}

type Repository interface {
	FindClient(context.Context, int64) (*domainv2.Client, error)
	FindOrCreatePeriod(context.Context, time.Time) (*domainv2.BillingPeriod, error)
	HasActiveInvoiceInPeriod(context.Context, int64, int64) (bool, error)
	HasLaterActiveInvoice(context.Context, int64, time.Time) (bool, error)
	FindActiveProducts(context.Context, []int64) ([]domainv2.Product, error)
	CreateInvoice(context.Context, *domainv2.Invoice) error
	CreateItems(context.Context, []domainv2.InvoiceItem) error
	PersistedSaleTotal(context.Context, int64) (decimal.Decimal, error)
	FindInvoice(context.Context, int64) (*invoiceRecord, error)
	FindItems(context.Context, int64) ([]domainv2.InvoiceItem, error)
	LatestActiveInvoiceID(context.Context, int64) (int64, error)
	CancelInvoice(context.Context, int64, time.Time, string) error
	ReverseActiveAllocations(context.Context, int64, time.Time, string) error
}

type QueryRepository interface {
	FindInvoiceClientID(context.Context, int64) (int64, error)
	FindInvoice(context.Context, int64) (*invoiceRecord, error)
	FindItems(context.Context, int64) ([]domainv2.InvoiceItem, error)
	ListInvoices(context.Context, listQuery) ([]invoiceRecord, error)
}

type Store struct{ db *gorm.DB }

func NewStore(db *gorm.DB) (*Store, error) {
	if db == nil {
		return nil, errors.New("invoice v2 database is required")
	}
	return &Store{db: db}, nil
}

func (store *Store) WithLockedClient(ctx context.Context, clientID int64, callback func(Repository, accountv2.AccountRepository) error) error {
	if clientID <= 0 || callback == nil {
		return ErrInvalidRequest
	}
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var client domainv2.Client
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Take(&client, clientID).Error; err != nil {
			return err
		}
		accountRepository, err := accountv2.NewTransactionRepository(tx)
		if err != nil {
			return err
		}
		return callback(&transactionRepository{db: tx}, accountRepository)
	})
}

type transactionRepository struct{ db *gorm.DB }

func (repository *transactionRepository) FindClient(ctx context.Context, id int64) (*domainv2.Client, error) {
	var client domainv2.Client
	if err := repository.db.WithContext(ctx).Take(&client, id).Error; err != nil {
		return nil, err
	}
	return &client, nil
}

func (repository *transactionRepository) FindOrCreatePeriod(ctx context.Context, period time.Time) (*domainv2.BillingPeriod, error) {
	var result domainv2.BillingPeriod
	err := repository.db.WithContext(ctx).Raw(`
INSERT INTO billing_periods (period_start)
VALUES (?)
ON CONFLICT (period_start) DO UPDATE SET period_start = EXCLUDED.period_start
RETURNING id, period_start, created_at`, period).Scan(&result).Error
	return &result, err
}

func (repository *transactionRepository) HasActiveInvoiceInPeriod(ctx context.Context, clientID, periodID int64) (bool, error) {
	var count int64
	err := repository.db.WithContext(ctx).Model(&domainv2.Invoice{}).
		Where("client_id = ? AND billing_period_id = ? AND status = 'issued'", clientID, periodID).Count(&count).Error
	return count > 0, err
}

func (repository *transactionRepository) HasLaterActiveInvoice(ctx context.Context, clientID int64, period time.Time) (bool, error) {
	var count int64
	err := repository.db.WithContext(ctx).Raw(`
SELECT COUNT(*)
FROM invoices i
JOIN billing_periods bp ON bp.id = i.billing_period_id
WHERE i.client_id = ? AND i.status = 'issued' AND bp.period_start > ?`, clientID, period).Scan(&count).Error
	return count > 0, err
}

func (repository *transactionRepository) FindActiveProducts(ctx context.Context, ids []int64) ([]domainv2.Product, error) {
	products := make([]domainv2.Product, 0, len(ids))
	err := repository.db.WithContext(ctx).Where("id IN ? AND active = TRUE", ids).Order("id").Find(&products).Error
	return products, err
}

func (repository *transactionRepository) CreateInvoice(ctx context.Context, invoice *domainv2.Invoice) error {
	return repository.db.WithContext(ctx).Omit("Items").Create(invoice).Error
}

func (repository *transactionRepository) CreateItems(ctx context.Context, items []domainv2.InvoiceItem) error {
	if len(items) == 0 {
		return ErrInvalidRequest
	}
	return repository.db.WithContext(ctx).Create(&items).Error
}

func (repository *transactionRepository) PersistedSaleTotal(ctx context.Context, invoiceID int64) (decimal.Decimal, error) {
	var row struct{ Total decimal.Decimal }
	err := repository.db.WithContext(ctx).Raw(`SELECT COALESCE(SUM(sale_total), 0) AS total FROM invoice_items WHERE invoice_id = ?`, invoiceID).Scan(&row).Error
	return row.Total, err
}

func (repository *transactionRepository) FindInvoice(ctx context.Context, id int64) (*invoiceRecord, error) {
	return findInvoice(ctx, repository.db, id)
}

func (repository *transactionRepository) FindItems(ctx context.Context, invoiceID int64) ([]domainv2.InvoiceItem, error) {
	return findItems(ctx, repository.db, invoiceID)
}

func (repository *transactionRepository) LatestActiveInvoiceID(ctx context.Context, clientID int64) (int64, error) {
	var row struct{ ID int64 }
	result := repository.db.WithContext(ctx).Raw(`
SELECT i.id
FROM invoices i
JOIN billing_periods bp ON bp.id = i.billing_period_id
WHERE i.client_id = ? AND i.status = 'issued'
ORDER BY bp.period_start DESC, i.created_at DESC, i.id DESC
LIMIT 1`, clientID).Scan(&row)
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	return row.ID, nil
}

func (repository *transactionRepository) CancelInvoice(ctx context.Context, invoiceID int64, at time.Time, reason string) error {
	result := repository.db.WithContext(ctx).Model(&domainv2.Invoice{}).
		Where("id = ? AND status = 'issued'", invoiceID).
		Updates(map[string]any{"status": "canceled", "canceled_at": at, "cancellation_reason": reason, "updated_at": at})
	return affected(result)
}

func (repository *transactionRepository) ReverseActiveAllocations(ctx context.Context, invoiceID int64, at time.Time, reason string) error {
	return repository.db.WithContext(ctx).Model(&domainv2.PaymentAllocation{}).
		Where("invoice_id = ? AND status = 'active'", invoiceID).
		Updates(map[string]any{"status": "reversed", "reversed_at": at, "reversal_kind": "invoice_cancellation", "reversal_reason": reason}).Error
}

func (store *Store) FindInvoiceClientID(ctx context.Context, invoiceID int64) (int64, error) {
	var row struct{ ClientID int64 }
	result := store.db.WithContext(ctx).Model(&domainv2.Invoice{}).Select("client_id").Where("id = ?", invoiceID).Scan(&row)
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	return row.ClientID, nil
}

func (store *Store) FindInvoice(ctx context.Context, id int64) (*invoiceRecord, error) {
	return findInvoice(ctx, store.db, id)
}

func (store *Store) FindItems(ctx context.Context, invoiceID int64) ([]domainv2.InvoiceItem, error) {
	return findItems(ctx, store.db, invoiceID)
}

type listQuery struct {
	PeriodStart time.Time
	ClientID    *int64
	Cursor      *listCursor
	Limit       int
}

func (store *Store) ListInvoices(ctx context.Context, query listQuery) ([]invoiceRecord, error) {
	where := []string{"bp.period_start = ?"}
	args := []any{query.PeriodStart}
	if query.ClientID != nil {
		where = append(where, "i.client_id = ?")
		args = append(args, *query.ClientID)
	}
	if query.Cursor != nil {
		where = append(where, "(i.created_at, i.id) < (?, ?)")
		args = append(args, query.Cursor.CreatedAt, query.Cursor.ID)
	}
	args = append(args, query.Limit)
	rows := make([]invoiceRecord, 0, query.Limit)
	err := store.db.WithContext(ctx).Raw(invoiceProjectionSQL+" WHERE "+strings.Join(where, " AND ")+invoiceProjectionGroup+`
ORDER BY i.created_at DESC, i.id DESC
LIMIT ?`, args...).Scan(&rows).Error
	return rows, err
}

func findInvoice(ctx context.Context, db *gorm.DB, id int64) (*invoiceRecord, error) {
	var row invoiceRecord
	result := db.WithContext(ctx).Raw(invoiceProjectionSQL+" WHERE i.id = ?"+invoiceProjectionGroup, id).Scan(&row)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &row, nil
}

func findItems(ctx context.Context, db *gorm.DB, invoiceID int64) ([]domainv2.InvoiceItem, error) {
	items := make([]domainv2.InvoiceItem, 0)
	err := db.WithContext(ctx).Where("invoice_id = ?", invoiceID).Order("id").Find(&items).Error
	return items, err
}

func affected(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

type invoiceRecord struct {
	ID                        int64
	Status                    string
	PeriodStart               time.Time
	ClientID                  int64
	ClientName                string
	ClientActive              bool
	ProductsTotal             decimal.Decimal
	AccountBalanceBeforeIssue decimal.Decimal
	AccountBalanceAfterCharge decimal.Decimal
	PaidAmount                decimal.Decimal
	OpenAmount                decimal.Decimal
	Observation               *string
	CreatedAt                 time.Time
	CanceledAt                *time.Time
	CancellationReason        *string
}

const invoiceProjectionSQL = `
SELECT i.id, i.status, bp.period_start, c.id AS client_id, c.name AS client_name, c.active AS client_active,
       i.products_total, i.account_balance_before_issue, i.account_balance_after_charge,
       CASE WHEN i.status = 'canceled' THEN 0 ELSE COALESCE(SUM(pa.amount) FILTER (WHERE pa.status = 'active'), 0) END AS paid_amount,
       CASE WHEN i.status = 'canceled' THEN 0 ELSE i.products_total - COALESCE(SUM(pa.amount) FILTER (WHERE pa.status = 'active'), 0) END AS open_amount,
       i.observation, i.created_at, i.canceled_at, i.cancellation_reason
FROM invoices i
JOIN billing_periods bp ON bp.id = i.billing_period_id
JOIN clients c ON c.id = i.client_id
LEFT JOIN payment_allocations pa ON pa.invoice_id = i.id`

const invoiceProjectionGroup = `
GROUP BY i.id, i.status, bp.period_start, c.id, c.name, c.active, i.products_total,
         i.account_balance_before_issue, i.account_balance_after_charge, i.observation,
         i.created_at, i.canceled_at, i.cancellation_reason`
