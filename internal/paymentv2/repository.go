package paymentv2

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
	CreatePayment(context.Context, *domainv2.Payment) error
	FindPayment(context.Context, int64) (*paymentRecord, error)
	FindActiveAllocations(context.Context, int64) ([]accountv2.Allocation, error)
	ReversePayment(context.Context, int64, time.Time, string) error
	ReverseActiveAllocations(context.Context, int64, time.Time, string) error
	ReverseOtherActiveAllocations(context.Context, int64, int64, time.Time, string) error
}

type QueryRepository interface {
	FindPaymentClientID(context.Context, int64) (int64, error)
	FindPayment(context.Context, int64) (*paymentRecord, error)
	FindActiveAllocations(context.Context, int64) ([]accountv2.Allocation, error)
	ListPayments(context.Context, listQuery) ([]paymentRecord, error)
}

type Store struct{ db *gorm.DB }

func NewStore(db *gorm.DB) (*Store, error) {
	if db == nil {
		return nil, errors.New("payment v2 database is required")
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

func (repository *transactionRepository) CreatePayment(ctx context.Context, payment *domainv2.Payment) error {
	return repository.db.WithContext(ctx).Create(payment).Error
}

func (repository *transactionRepository) FindPayment(ctx context.Context, id int64) (*paymentRecord, error) {
	return findPayment(ctx, repository.db, id)
}

func (repository *transactionRepository) FindActiveAllocations(ctx context.Context, paymentID int64) ([]accountv2.Allocation, error) {
	return findActiveAllocations(ctx, repository.db, paymentID)
}

func (repository *transactionRepository) ReversePayment(ctx context.Context, paymentID int64, at time.Time, reason string) error {
	return affected(repository.db.WithContext(ctx).Model(&domainv2.Payment{}).
		Where("id = ? AND status = 'posted'", paymentID).
		Updates(map[string]any{"status": "reversed", "reversed_at": at, "reversal_reason": reason}))
}

func (repository *transactionRepository) ReverseActiveAllocations(ctx context.Context, paymentID int64, at time.Time, reason string) error {
	return repository.db.WithContext(ctx).Model(&domainv2.PaymentAllocation{}).
		Where("payment_id = ? AND status = 'active'", paymentID).
		Updates(map[string]any{"status": "reversed", "reversed_at": at, "reversal_kind": "payment_reversal", "reversal_reason": reason}).Error
}

// ReverseOtherActiveAllocations clears the current settlement view before a
// payment reversal is reconciled. Keeping allocations from later payments in
// place could leave them attached to newer invoices while an older invoice was
// reopened, violating the account-wide FIFO rule.
func (repository *transactionRepository) ReverseOtherActiveAllocations(ctx context.Context, clientID, paymentID int64, at time.Time, reason string) error {
	return repository.db.WithContext(ctx).Model(&domainv2.PaymentAllocation{}).
		Where("client_id = ? AND payment_id <> ? AND status = 'active'", clientID, paymentID).
		Updates(map[string]any{"status": "reversed", "reversed_at": at, "reversal_kind": "fifo_reallocation", "reversal_reason": reason}).Error
}

func (store *Store) FindPaymentClientID(ctx context.Context, paymentID int64) (int64, error) {
	var row struct{ ClientID int64 }
	result := store.db.WithContext(ctx).Model(&domainv2.Payment{}).Select("client_id").Where("id = ?", paymentID).Scan(&row)
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	return row.ClientID, nil
}

func (store *Store) FindPayment(ctx context.Context, id int64) (*paymentRecord, error) {
	return findPayment(ctx, store.db, id)
}

func (store *Store) FindActiveAllocations(ctx context.Context, paymentID int64) ([]accountv2.Allocation, error) {
	return findActiveAllocations(ctx, store.db, paymentID)
}

type listQuery struct {
	ClientID *int64
	DateFrom *time.Time
	DateTo   *time.Time
	Status   string
	Cursor   *listCursor
	Limit    int
}

func (store *Store) ListPayments(ctx context.Context, query listQuery) ([]paymentRecord, error) {
	where := make([]string, 0, 5)
	args := make([]any, 0, 8)
	if query.ClientID != nil {
		where, args = append(where, "p.client_id = ?"), append(args, *query.ClientID)
	}
	if query.DateFrom != nil {
		where, args = append(where, "p.effective_date >= ?"), append(args, *query.DateFrom)
	}
	if query.DateTo != nil {
		where, args = append(where, "p.effective_date <= ?"), append(args, *query.DateTo)
	}
	if query.Status != "" {
		where, args = append(where, "p.status = ?"), append(args, query.Status)
	}
	if query.Cursor != nil {
		where = append(where, "(p.effective_date, p.created_at, p.id) < (?, ?, ?)")
		args = append(args, query.Cursor.EffectiveDate, query.Cursor.CreatedAt, query.Cursor.ID)
	}
	querySQL := paymentProjectionSQL
	if len(where) > 0 {
		querySQL += " WHERE " + strings.Join(where, " AND ")
	}
	args = append(args, query.Limit)
	rows := make([]paymentRecord, 0, query.Limit)
	err := store.db.WithContext(ctx).Raw(querySQL+paymentProjectionGroup+`
ORDER BY p.effective_date DESC, p.created_at DESC, p.id DESC
LIMIT ?`, args...).Scan(&rows).Error
	return rows, err
}

func findPayment(ctx context.Context, db *gorm.DB, id int64) (*paymentRecord, error) {
	var row paymentRecord
	result := db.WithContext(ctx).Raw(paymentProjectionSQL+" WHERE p.id = ?"+paymentProjectionGroup, id).Scan(&row)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &row, nil
}

func findActiveAllocations(ctx context.Context, db *gorm.DB, paymentID int64) ([]accountv2.Allocation, error) {
	rows := make([]accountv2.Allocation, 0)
	err := db.WithContext(ctx).Model(&domainv2.PaymentAllocation{}).
		Select("payment_id, invoice_id, amount").Where("payment_id = ? AND status = 'active'", paymentID).
		Order("allocated_at, id").Scan(&rows).Error
	return rows, err
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

type paymentRecord struct {
	ID              int64
	ClientID        int64
	ClientName      string
	ClientActive    bool
	Amount          decimal.Decimal
	EffectiveDate   time.Time
	Observation     *string
	Status          string
	AllocatedAmount decimal.Decimal
	CreatedAt       time.Time
	ReversedAt      *time.Time
	ReversalReason  *string
}

const paymentProjectionSQL = `
SELECT p.id, c.id AS client_id, c.name AS client_name, c.active AS client_active,
       p.amount, p.effective_date, p.observation, p.status,
       CASE WHEN p.status = 'reversed' THEN 0 ELSE COALESCE(SUM(pa.amount) FILTER (WHERE pa.status = 'active'), 0) END AS allocated_amount,
       p.created_at, p.reversed_at, p.reversal_reason
FROM payments p
JOIN clients c ON c.id = p.client_id
LEFT JOIN payment_allocations pa ON pa.payment_id = p.id`

const paymentProjectionGroup = `
GROUP BY p.id, c.id, c.name, c.active, p.amount, p.effective_date, p.observation,
         p.status, p.created_at, p.reversed_at, p.reversal_reason`
