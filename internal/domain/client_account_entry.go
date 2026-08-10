package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

const (
	AccountEntryCharge  = "charge"
	AccountEntryPayment = "payment"
)

type ClientAccountEntry struct {
	ID          int64           `gorm:"primaryKey;autoIncrement;not null"`
	DateCreated time.Time       `gorm:"not null;default:current_timestamp"`
	ClientID    int64           `gorm:"not null;index"`
	OrderID     *int64          `gorm:"index"`
	EntryType   string          `gorm:"not null"`
	Amount      decimal.Decimal `gorm:"type:numeric(14,2);not null"`
	OrderYear   int16           `gorm:"not null"`
	OrderMonth  int16           `gorm:"not null"`
}

func (ClientAccountEntry) TableName() string { return "client_account_entry" }
