package paymentv2

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/api-control/internal/accountv2"
)

const (
	defaultLimit = 50
	maxLimit     = 100
)

var (
	ErrInvalidRequest      = errors.New("invalid payment request")
	ErrInvalidFilter       = errors.New("invalid payment filter")
	ErrFutureEffectiveDate = errors.New("future payment effective date")
	ErrPaymentNotPosted    = errors.New("payment is not posted")
	ErrInvalidCursor       = errors.New("invalid payment cursor")
)

type CreateRequest struct {
	ClientID      int64                 `json:"clientId"`
	Amount        *accountv2.JSONAmount `json:"amount"`
	EffectiveDate string                `json:"effectiveDate"`
	Observation   *string               `json:"observation"`
}

type ReverseRequest struct {
	Reason string `json:"reason"`
}

type ListFilter struct {
	ClientID *int64
	DateFrom *time.Time
	DateTo   *time.Time
	Status   string
	Cursor   string
	Limit    int
}

type ClientSummary struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type AllocationResponse struct {
	InvoiceID int64                `json:"invoiceId"`
	Amount    accountv2.JSONAmount `json:"amount"`
}

type AccountResponse struct {
	Position     string               `json:"position"`
	NetBalance   accountv2.JSONAmount `json:"netBalance"`
	DebtAmount   accountv2.JSONAmount `json:"debtAmount"`
	CreditAmount accountv2.JSONAmount `json:"creditAmount"`
}

type PaymentResponse struct {
	ID              int64                `json:"id"`
	Client          ClientSummary        `json:"client"`
	Amount          accountv2.JSONAmount `json:"amount"`
	EffectiveDate   string               `json:"effectiveDate"`
	Observation     *string              `json:"observation,omitempty"`
	Status          string               `json:"status"`
	AllocatedAmount accountv2.JSONAmount `json:"allocatedAmount"`
	CreditAmount    accountv2.JSONAmount `json:"creditAmount"`
	Allocations     []AllocationResponse `json:"allocations"`
	CreatedAt       time.Time            `json:"createdAt"`
	ReversedAt      *time.Time           `json:"reversedAt,omitempty"`
	ReversalReason  *string              `json:"reversalReason,omitempty"`
}

type MutationResponse struct {
	PaymentResponse
	Account AccountResponse `json:"account"`
}

type ListResponse struct {
	Items      []PaymentResponse `json:"items"`
	NextCursor *string           `json:"nextCursor"`
}

type listCursor struct {
	EffectiveDate time.Time `json:"effectiveDate"`
	CreatedAt     time.Time `json:"createdAt"`
	ID            int64     `json:"id"`
}

func encodeCursor(row paymentRecord) string {
	data, _ := json.Marshal(listCursor{EffectiveDate: row.EffectiveDate, CreatedAt: row.CreatedAt, ID: row.ID})
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
	if err := decoder.Decode(&cursor); err != nil || cursor.ID <= 0 || cursor.EffectiveDate.IsZero() || cursor.CreatedAt.IsZero() {
		return nil, ErrInvalidCursor
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, ErrInvalidCursor
	}
	return &cursor, nil
}
