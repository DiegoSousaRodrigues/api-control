package invoicev2

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/api-control/internal/accountv2"
	"github.com/shopspring/decimal"
)

const (
	defaultLimit = 50
	maxLimit     = 100
	maxQuantity  = int(^uint32(0) >> 1)
)

var (
	ErrInvalidRequest       = errors.New("invalid invoice request")
	ErrFuturePeriod         = errors.New("future billing period is not allowed")
	ErrLaterInvoiceExists   = errors.New("a later active invoice exists")
	ErrInvoiceAlreadyExists = errors.New("an active invoice already exists for this client and period")
	ErrInactiveClient       = errors.New("client must be active")
	ErrInactiveProduct      = errors.New("every product must exist and be active")
	ErrInvoiceNotIssued     = errors.New("invoice is not issued")
	ErrInvoiceNotLatest     = errors.New("only the latest active invoice can be canceled")
	ErrInvalidCursor        = errors.New("invalid invoice cursor")
	ErrPersistedTotal       = errors.New("persisted invoice item total does not match invoice total")
)

type IssueRequest struct {
	ClientID    int64          `json:"clientId"`
	Year        int            `json:"year"`
	Month       int            `json:"month"`
	Observation *string        `json:"observation"`
	Products    []IssueProduct `json:"products"`
}

type IssueProduct struct {
	ProductID int64 `json:"productId"`
	Quantity  int   `json:"quantity"`
}

type CancelRequest struct {
	Reason string `json:"reason"`
}

type ListFilter struct {
	Year     int
	Month    int
	ClientID *int64
	Cursor   string
	Limit    int
}

type Amount decimal.Decimal

func newAmount(value decimal.Decimal) Amount { return Amount(value.Copy()) }

func (amount Amount) Decimal() decimal.Decimal { return decimal.Decimal(amount) }

func (amount Amount) MarshalJSON() ([]byte, error) {
	value := amount.Decimal()
	if value.Abs().GreaterThan(decimal.RequireFromString("9999999999999.99")) {
		return nil, errors.New("amount is outside NUMERIC(15,2) range")
	}
	return []byte(value.StringFixed(2)), nil
}

type PeriodResponse struct {
	Year  int `json:"year"`
	Month int `json:"month"`
}

type ClientSummary struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type InvoiceSummary struct {
	ID                        int64          `json:"id"`
	Status                    string         `json:"status"`
	Period                    PeriodResponse `json:"period"`
	Client                    ClientSummary  `json:"client"`
	ProductsTotal             Amount         `json:"productsTotal"`
	AccountBalanceBeforeIssue Amount         `json:"accountBalanceBeforeIssue"`
	AccountBalanceAfterCharge Amount         `json:"accountBalanceAfterCharge"`
	PaidAmount                Amount         `json:"paidAmount"`
	OpenAmount                Amount         `json:"openAmount"`
	PaymentStatus             string         `json:"paymentStatus"`
	Observation               *string        `json:"observation,omitempty"`
	CreatedAt                 time.Time      `json:"createdAt"`
	CanceledAt                *time.Time     `json:"canceledAt,omitempty"`
	CancellationReason        *string        `json:"cancellationReason,omitempty"`
}

type InvoiceItemResponse struct {
	ID                int64  `json:"id"`
	ProductID         int64  `json:"productId"`
	ProductName       string `json:"productName"`
	Quantity          int    `json:"quantity"`
	UnitPurchasePrice Amount `json:"unitPurchasePrice"`
	UnitSalePrice     Amount `json:"unitSalePrice"`
	PurchaseTotal     Amount `json:"purchaseTotal"`
	SaleTotal         Amount `json:"saleTotal"`
	ProfitTotal       Amount `json:"profitTotal"`
}

type InvoiceDetail struct {
	InvoiceSummary
	Items []InvoiceItemResponse `json:"items"`
}

type AccountResponse struct {
	Position     string `json:"position"`
	NetBalance   Amount `json:"netBalance"`
	DebtAmount   Amount `json:"debtAmount"`
	CreditAmount Amount `json:"creditAmount"`
}

type MutationResponse struct {
	Invoice InvoiceDetail   `json:"invoice"`
	Account AccountResponse `json:"account"`
}

type ListResponse struct {
	Items      []InvoiceSummary `json:"items"`
	NextCursor *string          `json:"nextCursor"`
}

type listCursor struct {
	CreatedAt time.Time `json:"createdAt"`
	ID        int64     `json:"id"`
}

func encodeCursor(createdAt time.Time, id int64) string {
	data, _ := json.Marshal(listCursor{CreatedAt: createdAt, ID: id})
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeCursor(value string) (*listCursor, error) {
	if value == "" {
		return nil, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, ErrInvalidCursor
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var cursor listCursor
	if err := decoder.Decode(&cursor); err != nil || cursor.ID <= 0 || cursor.CreatedAt.IsZero() {
		return nil, ErrInvalidCursor
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, ErrInvalidCursor
	}
	return &cursor, nil
}

func accountResponse(position accountv2.Position) AccountResponse {
	return AccountResponse{Position: position.State, NetBalance: newAmount(position.NetBalance),
		DebtAmount: newAmount(position.DebtAmount), CreditAmount: newAmount(position.CreditAmount)}
}
