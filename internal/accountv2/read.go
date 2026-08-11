package accountv2

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	statementDefaultLimit = 50
	statementMaxLimit     = 100
)

var (
	ErrInvalidReadRequest     = errors.New("invalid account read request")
	ErrInvalidStatementCursor = errors.New("invalid statement cursor")
)

type AccountClient struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type AccountSummaryResponse struct {
	Client           AccountClient `json:"client"`
	Position         string        `json:"position"`
	NetBalance       JSONAmount    `json:"netBalance"`
	DebtAmount       JSONAmount    `json:"debtAmount"`
	CreditAmount     JSONAmount    `json:"creditAmount"`
	OpenInvoiceCount int64         `json:"openInvoiceCount"`
	AsOf             time.Time     `json:"asOf"`
}

type StatementItem struct {
	EventID           string     `json:"eventId"`
	Type              string     `json:"type"`
	EffectiveDate     string     `json:"effectiveDate"`
	RecordedAt        time.Time  `json:"recordedAt"`
	InvoiceID         *int64     `json:"invoiceId"`
	PaymentID         *int64     `json:"paymentId"`
	Description       string     `json:"description"`
	Debit             JSONAmount `json:"debit"`
	Credit            JSONAmount `json:"credit"`
	BalanceAfterEvent JSONAmount `json:"balanceAfterEvent"`
}

type StatementResponse struct {
	Items              []StatementItem `json:"items"`
	NextCursor         *string         `json:"nextCursor"`
	SnapshotRecordedAt time.Time       `json:"snapshotRecordedAt"`
}

type StatementFilter struct {
	Cursor string
	Limit  int
	DateTo *time.Time
}

type summaryRecord struct {
	ClientID         int64
	ClientName       string
	ClientActive     bool
	NetBalance       decimal.Decimal
	OpenInvoiceCount int64
}

type statementRecord struct {
	EventID           string
	Type              string
	EffectiveDate     time.Time
	RecordedAt        time.Time
	InvoiceID         *int64
	PaymentID         *int64
	Description       string
	Debit             decimal.Decimal
	Credit            decimal.Decimal
	BalanceAfterEvent decimal.Decimal
	SourceID          int64
}

type statementQuery struct {
	ClientID int64
	Cutoff   time.Time
	DateTo   *time.Time
	Cursor   *statementCursor
	Limit    int
}

type ReadRepository interface {
	Summary(context.Context, int64) (*summaryRecord, error)
	ClientExists(context.Context, int64) (bool, error)
	Statement(context.Context, statementQuery) ([]statementRecord, error)
}

func (store *Store) Summary(ctx context.Context, clientID int64) (*summaryRecord, error) {
	var row summaryRecord
	result := store.db.WithContext(ctx).Raw(accountSummarySQL, clientID).Scan(&row)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &row, nil
}

func (store *Store) ClientExists(ctx context.Context, clientID int64) (bool, error) {
	var count int64
	err := store.db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM clients WHERE id = ?`, clientID).Scan(&count).Error
	return count > 0, err
}

func (store *Store) Statement(ctx context.Context, query statementQuery) ([]statementRecord, error) {
	outerWhere := ""
	args := []any{sql.Named("clientID", query.ClientID), sql.Named("cutoff", query.Cutoff)}
	if query.DateTo != nil {
		outerWhere = " WHERE effective_date <= @dateTo"
		args = append(args, sql.Named("dateTo", *query.DateTo))
	}
	runningWhere := ""
	if query.Cursor != nil {
		runningWhere = " WHERE (effective_date, recorded_at, type, source_id) < (@effectiveDate, @recordedAt, @type, @sourceID)"
		args = append(args, sql.Named("effectiveDate", query.Cursor.EffectiveDate), sql.Named("recordedAt", query.Cursor.RecordedAt),
			sql.Named("type", query.Cursor.Type), sql.Named("sourceID", query.Cursor.SourceID))
	}
	args = append(args, sql.Named("limit", query.Limit))
	rows := make([]statementRecord, 0, query.Limit)
	err := store.db.WithContext(ctx).Raw(statementEventsSQL+outerWhere+statementRunningSQL+runningWhere+`
ORDER BY effective_date DESC, recorded_at DESC, type DESC, source_id DESC
LIMIT @limit`, args...).Scan(&rows).Error
	return rows, err
}

type ReadService struct {
	repository ReadRepository
	now        func() time.Time
	location   *time.Location
}

func NewReadService(repository ReadRepository, now func() time.Time) (*ReadService, error) {
	if repository == nil {
		return nil, errors.New("account v2 read repository is required")
	}
	if now == nil {
		now = time.Now
	}
	location, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return nil, err
	}
	return &ReadService{repository: repository, now: now, location: location}, nil
}

func (service *ReadService) Summary(ctx context.Context, clientID int64) (*AccountSummaryResponse, error) {
	if clientID <= 0 {
		return nil, ErrInvalidReadRequest
	}
	record, err := service.repository.Summary(ctx, clientID)
	if err != nil {
		return nil, err
	}
	position := NewPosition(clientID, record.NetBalance)
	return &AccountSummaryResponse{Client: AccountClient{ID: record.ClientID, Name: record.ClientName, Active: record.ClientActive},
		Position: position.State, NetBalance: NewJSONAmount(position.NetBalance), DebtAmount: NewJSONAmount(position.DebtAmount),
		CreditAmount: NewJSONAmount(position.CreditAmount), OpenInvoiceCount: record.OpenInvoiceCount,
		AsOf: service.now().In(service.location)}, nil
}

func (service *ReadService) Statement(ctx context.Context, clientID int64, filter StatementFilter) (*StatementResponse, error) {
	if clientID <= 0 || filter.Limit < 0 || filter.Limit > statementMaxLimit {
		return nil, ErrInvalidReadRequest
	}
	exists, err := service.repository.ClientExists(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, gorm.ErrRecordNotFound
	}
	cursor, err := decodeStatementCursor(filter.Cursor)
	if err != nil {
		return nil, err
	}
	dateToKey := ""
	if filter.DateTo != nil {
		dateToKey = filter.DateTo.Format("2006-01-02")
	}
	cutoff := service.now().UTC()
	if cursor != nil {
		if cursor.ClientID != clientID || cursor.DateTo != dateToKey || cursor.Cutoff.After(cutoff) {
			return nil, ErrInvalidStatementCursor
		}
		cutoff = cursor.Cutoff
	}
	limit := filter.Limit
	if limit == 0 {
		limit = statementDefaultLimit
	}
	records, err := service.repository.Statement(ctx, statementQuery{ClientID: clientID, Cutoff: cutoff,
		DateTo: filter.DateTo, Cursor: cursor, Limit: limit + 1})
	if err != nil {
		return nil, err
	}
	response := &StatementResponse{Items: make([]StatementItem, 0, min(len(records), limit)), SnapshotRecordedAt: cutoff.In(service.location)}
	visible := records
	if len(records) > limit {
		visible = records[:limit]
		last := visible[len(visible)-1]
		next := encodeStatementCursor(statementCursor{ClientID: clientID, Cutoff: cutoff, EffectiveDate: last.EffectiveDate,
			RecordedAt: last.RecordedAt, Type: last.Type, SourceID: last.SourceID, DateTo: dateToKey})
		response.NextCursor = &next
	}
	for _, record := range visible {
		response.Items = append(response.Items, StatementItem{EventID: record.EventID, Type: record.Type,
			EffectiveDate: record.EffectiveDate.Format("2006-01-02"), RecordedAt: record.RecordedAt,
			InvoiceID: record.InvoiceID, PaymentID: record.PaymentID, Description: record.Description,
			Debit: NewJSONAmount(record.Debit), Credit: NewJSONAmount(record.Credit),
			BalanceAfterEvent: NewJSONAmount(record.BalanceAfterEvent)})
	}
	return response, nil
}

type statementCursor struct {
	ClientID      int64     `json:"clientId"`
	Cutoff        time.Time `json:"cutoff"`
	EffectiveDate time.Time `json:"effectiveDate"`
	RecordedAt    time.Time `json:"recordedAt"`
	Type          string    `json:"type"`
	SourceID      int64     `json:"sourceId"`
	DateTo        string    `json:"dateTo"`
}

func encodeStatementCursor(cursor statementCursor) string {
	data, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeStatementCursor(value string) (*statementCursor, error) {
	if value == "" {
		return nil, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, ErrInvalidStatementCursor
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var cursor statementCursor
	if err := decoder.Decode(&cursor); err != nil || cursor.ClientID <= 0 || cursor.Cutoff.IsZero() || cursor.EffectiveDate.IsZero() ||
		cursor.RecordedAt.IsZero() || !validStatementType(cursor.Type) || cursor.SourceID <= 0 {
		return nil, ErrInvalidStatementCursor
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, ErrInvalidStatementCursor
	}
	return &cursor, nil
}

func validStatementType(value string) bool {
	switch value {
	case "invoice_issued", "invoice_canceled", "payment_posted", "payment_reversed":
		return true
	default:
		return false
	}
}

const accountSummarySQL = `
SELECT c.id AS client_id, c.name AS client_name, c.active AS client_active,
       COALESCE((SELECT SUM(products_total) FROM invoices WHERE client_id = c.id AND status = 'issued'), 0)
       - COALESCE((SELECT SUM(amount) FROM payments WHERE client_id = c.id AND status = 'posted'), 0) AS net_balance,
       (SELECT COUNT(*) FROM invoices i
        WHERE i.client_id = c.id AND i.status = 'issued'
          AND i.products_total - COALESCE((SELECT SUM(pa.amount) FROM payment_allocations pa
                                           WHERE pa.invoice_id = i.id AND pa.status = 'active'), 0) > 0) AS open_invoice_count
FROM clients c WHERE c.id = ?`

const statementEventsSQL = `
WITH events AS (
    SELECT 'invoice:' || i.id AS event_id, 'invoice_issued' AS type,
           (i.created_at AT TIME ZONE 'America/Sao_Paulo')::date AS effective_date,
           i.created_at AS recorded_at, i.id AS invoice_id, NULL::bigint AS payment_id,
           'Fatura emitida' AS description, i.products_total AS debit, 0::numeric AS credit, i.id AS source_id
    FROM invoices i WHERE i.client_id = @clientID AND i.created_at <= @cutoff
    UNION ALL
    SELECT 'invoice_cancel:' || i.id, 'invoice_canceled',
           (i.canceled_at AT TIME ZONE 'America/Sao_Paulo')::date, i.canceled_at, i.id, NULL::bigint,
           'Fatura cancelada', 0::numeric, i.products_total, i.id
    FROM invoices i WHERE i.client_id = @clientID AND i.canceled_at IS NOT NULL AND i.canceled_at <= @cutoff
    UNION ALL
    SELECT 'payment:' || p.id, 'payment_posted', p.effective_date, p.created_at, NULL::bigint, p.id,
           'Pagamento registrado', 0::numeric, p.amount, p.id
    FROM payments p WHERE p.client_id = @clientID AND p.created_at <= @cutoff
    UNION ALL
    SELECT 'payment_reverse:' || p.id, 'payment_reversed',
           (p.reversed_at AT TIME ZONE 'America/Sao_Paulo')::date, p.reversed_at, NULL::bigint, p.id,
           'Pagamento estornado', p.amount, 0::numeric, p.id
    FROM payments p WHERE p.client_id = @clientID AND p.reversed_at IS NOT NULL AND p.reversed_at <= @cutoff
), bounded AS (
    SELECT * FROM events`

const statementRunningSQL = `
), running AS (
    SELECT *, SUM(debit - credit) OVER (
        ORDER BY effective_date ASC, recorded_at ASC, type ASC, source_id ASC
    ) AS balance_after_event
    FROM bounded
)
SELECT event_id, type, effective_date, recorded_at, invoice_id, payment_id, description,
       debit, credit, balance_after_event, source_id
FROM running`
