package accountv2

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

var maxJSONAmount = decimal.RequireFromString("9999999999999.99")

// JSONAmount is the HTTP boundary for NUMERIC(15,2). It always emits a JSON
// number and rejects strings, exponent notation and sub-cent precision.
type JSONAmount decimal.Decimal

func NewJSONAmount(value decimal.Decimal) JSONAmount { return JSONAmount(value.Copy()) }

func (amount JSONAmount) Decimal() decimal.Decimal { return decimal.Decimal(amount) }

func (amount JSONAmount) MarshalJSON() ([]byte, error) {
	value := amount.Decimal()
	if value.Exponent() < -2 || value.Abs().GreaterThan(maxJSONAmount) {
		return nil, errors.New("amount is outside NUMERIC(15,2) range")
	}
	return []byte(value.StringFixed(2)), nil
}

func (amount *JSONAmount) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) || data[0] == '"' || bytes.ContainsAny(data, "eE") {
		return errors.New("amount must be a JSON number")
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err != nil {
		return fmt.Errorf("amount must be a JSON number: %w", err)
	}
	value, err := decimal.NewFromString(number.String())
	if err != nil || value.Exponent() < -2 || value.Abs().GreaterThan(maxJSONAmount) {
		return errors.New("amount is outside NUMERIC(15,2) range")
	}
	*amount = NewJSONAmount(value)
	return nil
}

const (
	PositionDebt    = "debt"
	PositionSettled = "settled"
	PositionCredit  = "credit"
)

var (
	ErrInvalidClientID            = errors.New("client id must be positive")
	ErrInvalidPaymentID           = errors.New("payment id must be positive")
	ErrInvalidInvoiceID           = errors.New("invoice id must be positive")
	ErrInvalidAllocation          = errors.New("allocation must be positive and within NUMERIC(15,2)")
	ErrInvoiceOverallocated       = errors.New("active allocations exceed invoice total")
	ErrPaymentOverallocated       = errors.New("active allocations exceed payment amount")
	ErrInvalidActiveAllocation    = errors.New("active allocation references canceled invoice or reversed payment")
	ErrInconsistentAccountSession = errors.New("account operation references another client")
)

type Position struct {
	ClientID     int64
	NetBalance   decimal.Decimal
	DebtAmount   decimal.Decimal
	CreditAmount decimal.Decimal
	State        string
}

func NewPosition(clientID int64, netBalance decimal.Decimal) Position {
	position := Position{ClientID: clientID, NetBalance: netBalance.Copy(), State: PositionSettled,
		DebtAmount: decimal.Zero, CreditAmount: decimal.Zero}
	switch {
	case netBalance.IsPositive():
		position.State = PositionDebt
		position.DebtAmount = netBalance.Copy()
	case netBalance.IsNegative():
		position.State = PositionCredit
		position.CreditAmount = netBalance.Abs()
	}
	return position
}

type OpenInvoice struct {
	ID            int64
	ClientID      int64
	PeriodStart   time.Time
	CreatedAt     time.Time
	ProductsTotal decimal.Decimal
	PaidAmount    decimal.Decimal
	OpenAmount    decimal.Decimal
}

type PaymentCredit struct {
	ID                int64
	ClientID          int64
	EffectiveDate     time.Time
	CreatedAt         time.Time
	Amount            decimal.Decimal
	AllocatedAmount   decimal.Decimal
	UnallocatedAmount decimal.Decimal
}

type Allocation struct {
	PaymentID int64
	InvoiceID int64
	Amount    decimal.Decimal
}
