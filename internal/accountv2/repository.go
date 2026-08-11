package accountv2

import (
	"context"
	"errors"

	domainv2 "github.com/api-control/internal/domain/v2"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ClientUnitOfWork interface {
	WithLockedClient(context.Context, int64, func(AccountRepository) error) error
}

type AccountRepository interface {
	Position(context.Context, int64) (Position, error)
	OpenInvoicesFIFO(context.Context, int64) ([]OpenInvoice, error)
	InvoiceOpenAmount(context.Context, int64, int64) (OpenInvoice, error)
	AvailablePaymentsFIFO(context.Context, int64) ([]PaymentCredit, error)
	PaymentCredit(context.Context, int64, int64) (PaymentCredit, error)
	CreateAllocation(context.Context, int64, Allocation) error
	ValidateConservation(context.Context, int64) error
}

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) (*Store, error) {
	if db == nil {
		return nil, errors.New("account v2 database is required")
	}
	return &Store{db: db}, nil
}

func (store *Store) WithLockedClient(ctx context.Context, clientID int64, callback func(AccountRepository) error) error {
	if clientID <= 0 {
		return ErrInvalidClientID
	}
	if callback == nil {
		return errors.New("account transaction callback is required")
	}
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var client domainv2.Client
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Take(&client, clientID).Error; err != nil {
			return err
		}
		return callback(&transactionRepository{db: tx})
	})
}

type transactionRepository struct {
	db *gorm.DB
}

// NewTransactionRepository binds the account projection/allocation repository
// to an existing financial transaction. Callers must already hold the client
// row lock for the whole callback.
func NewTransactionRepository(tx *gorm.DB) (AccountRepository, error) {
	if tx == nil {
		return nil, errors.New("account v2 transaction is required")
	}
	return &transactionRepository{db: tx}, nil
}

func (repository *transactionRepository) Position(ctx context.Context, clientID int64) (Position, error) {
	var row struct {
		NetBalance decimal.Decimal
	}
	err := repository.db.WithContext(ctx).Raw(accountPositionSQL, clientID, clientID).Scan(&row).Error
	if err != nil {
		return Position{}, err
	}
	return NewPosition(clientID, row.NetBalance), nil
}

func (repository *transactionRepository) OpenInvoicesFIFO(ctx context.Context, clientID int64) ([]OpenInvoice, error) {
	rows := make([]OpenInvoice, 0)
	err := repository.db.WithContext(ctx).Raw(openInvoicesSQL, clientID).Scan(&rows).Error
	return rows, err
}

func (repository *transactionRepository) InvoiceOpenAmount(ctx context.Context, clientID, invoiceID int64) (OpenInvoice, error) {
	var row OpenInvoice
	result := repository.db.WithContext(ctx).Raw(invoiceOpenAmountSQL, clientID, invoiceID).Scan(&row)
	if result.Error != nil {
		return OpenInvoice{}, result.Error
	}
	if result.RowsAffected == 0 {
		return OpenInvoice{}, gorm.ErrRecordNotFound
	}
	return row, nil
}

func (repository *transactionRepository) AvailablePaymentsFIFO(ctx context.Context, clientID int64) ([]PaymentCredit, error) {
	rows := make([]PaymentCredit, 0)
	err := repository.db.WithContext(ctx).Raw(availablePaymentsSQL, clientID).Scan(&rows).Error
	return rows, err
}

func (repository *transactionRepository) PaymentCredit(ctx context.Context, clientID, paymentID int64) (PaymentCredit, error) {
	var row PaymentCredit
	result := repository.db.WithContext(ctx).Raw(paymentCreditSQL, clientID, paymentID).Scan(&row)
	if result.Error != nil {
		return PaymentCredit{}, result.Error
	}
	if result.RowsAffected == 0 {
		return PaymentCredit{}, gorm.ErrRecordNotFound
	}
	return row, nil
}

func (repository *transactionRepository) CreateAllocation(ctx context.Context, clientID int64, allocation Allocation) error {
	if clientID <= 0 || allocation.PaymentID <= 0 || allocation.InvoiceID <= 0 || !validAmount(allocation.Amount) {
		return ErrInvalidAllocation
	}
	return repository.db.WithContext(ctx).Create(&domainv2.PaymentAllocation{
		ClientID: clientID, PaymentID: allocation.PaymentID, InvoiceID: allocation.InvoiceID,
		Amount: allocation.Amount.Copy(), Status: "active",
	}).Error
}

func (repository *transactionRepository) ValidateConservation(ctx context.Context, clientID int64) error {
	var violations struct {
		InvoiceOverallocated    int64
		PaymentOverallocated    int64
		InvalidActiveAllocation int64
	}
	if err := repository.db.WithContext(ctx).Raw(conservationSQL, clientID, clientID, clientID).Scan(&violations).Error; err != nil {
		return err
	}
	switch {
	case violations.InvalidActiveAllocation > 0:
		return ErrInvalidActiveAllocation
	case violations.InvoiceOverallocated > 0:
		return ErrInvoiceOverallocated
	case violations.PaymentOverallocated > 0:
		return ErrPaymentOverallocated
	default:
		return nil
	}
}

func validAmount(amount decimal.Decimal) bool {
	max := decimal.RequireFromString("9999999999999.99")
	return amount.IsPositive() && amount.Exponent() >= -2 && amount.LessThanOrEqual(max)
}

const accountPositionSQL = `
SELECT
    COALESCE((SELECT SUM(products_total) FROM invoices WHERE client_id = ? AND status = 'issued'), 0)
    - COALESCE((SELECT SUM(amount) FROM payments WHERE client_id = ? AND status = 'posted'), 0)
    AS net_balance`

const openInvoicesSQL = `
SELECT
    i.id,
    i.client_id,
    bp.period_start,
    i.created_at,
    i.products_total,
    COALESCE(SUM(pa.amount) FILTER (WHERE pa.status = 'active'), 0) AS paid_amount,
    i.products_total - COALESCE(SUM(pa.amount) FILTER (WHERE pa.status = 'active'), 0) AS open_amount
FROM invoices i
JOIN billing_periods bp ON bp.id = i.billing_period_id
LEFT JOIN payment_allocations pa ON pa.invoice_id = i.id
WHERE i.client_id = ? AND i.status = 'issued'
GROUP BY i.id, i.client_id, bp.period_start, i.created_at, i.products_total
HAVING i.products_total - COALESCE(SUM(pa.amount) FILTER (WHERE pa.status = 'active'), 0) > 0
ORDER BY bp.period_start ASC, i.created_at ASC, i.id ASC`

const invoiceOpenAmountSQL = `
SELECT
    i.id,
    i.client_id,
    bp.period_start,
    i.created_at,
    i.products_total,
    COALESCE(SUM(pa.amount) FILTER (WHERE pa.status = 'active'), 0) AS paid_amount,
    i.products_total - COALESCE(SUM(pa.amount) FILTER (WHERE pa.status = 'active'), 0) AS open_amount
FROM invoices i
JOIN billing_periods bp ON bp.id = i.billing_period_id
LEFT JOIN payment_allocations pa ON pa.invoice_id = i.id
WHERE i.client_id = ? AND i.id = ? AND i.status = 'issued'
GROUP BY i.id, i.client_id, bp.period_start, i.created_at, i.products_total`

const availablePaymentsSQL = `
SELECT
    p.id,
    p.client_id,
    p.effective_date,
    p.created_at,
    p.amount,
    COALESCE(SUM(pa.amount) FILTER (WHERE pa.status = 'active'), 0) AS allocated_amount,
    p.amount - COALESCE(SUM(pa.amount) FILTER (WHERE pa.status = 'active'), 0) AS unallocated_amount
FROM payments p
LEFT JOIN payment_allocations pa ON pa.payment_id = p.id
WHERE p.client_id = ? AND p.status = 'posted'
GROUP BY p.id, p.client_id, p.effective_date, p.created_at, p.amount
HAVING p.amount - COALESCE(SUM(pa.amount) FILTER (WHERE pa.status = 'active'), 0) > 0
ORDER BY p.effective_date ASC, p.created_at ASC, p.id ASC`

const paymentCreditSQL = `
SELECT
    p.id,
    p.client_id,
    p.effective_date,
    p.created_at,
    p.amount,
    COALESCE(SUM(pa.amount) FILTER (WHERE pa.status = 'active'), 0) AS allocated_amount,
    p.amount - COALESCE(SUM(pa.amount) FILTER (WHERE pa.status = 'active'), 0) AS unallocated_amount
FROM payments p
LEFT JOIN payment_allocations pa ON pa.payment_id = p.id
WHERE p.client_id = ? AND p.id = ? AND p.status = 'posted'
GROUP BY p.id, p.client_id, p.effective_date, p.created_at, p.amount`

const conservationSQL = `
SELECT
    (
        SELECT COUNT(*) FROM (
            SELECT i.id
            FROM invoices i
            LEFT JOIN payment_allocations pa ON pa.invoice_id = i.id AND pa.status = 'active'
            WHERE i.client_id = ?
            GROUP BY i.id, i.products_total
            HAVING COALESCE(SUM(pa.amount), 0) > i.products_total
        ) invoice_violations
    ) AS invoice_overallocated,
    (
        SELECT COUNT(*) FROM (
            SELECT p.id
            FROM payments p
            LEFT JOIN payment_allocations pa ON pa.payment_id = p.id AND pa.status = 'active'
            WHERE p.client_id = ?
            GROUP BY p.id, p.amount
            HAVING COALESCE(SUM(pa.amount), 0) > p.amount
        ) payment_violations
    ) AS payment_overallocated,
    (
        SELECT COUNT(*)
        FROM payment_allocations pa
        JOIN invoices i ON i.id = pa.invoice_id
        JOIN payments p ON p.id = pa.payment_id
        WHERE pa.client_id = ? AND pa.status = 'active'
          AND (i.status <> 'issued' OR p.status <> 'posted' OR i.client_id <> pa.client_id OR p.client_id <> pa.client_id)
    ) AS invalid_active_allocation`
