package accountv2

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

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
