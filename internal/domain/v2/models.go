package v2

import (
	"time"

	"github.com/shopspring/decimal"
)

type User struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`
	Name         string    `gorm:"not null"`
	Login        string    `gorm:"not null;uniqueIndex"`
	PasswordHash string    `gorm:"not null"`
	Active       bool      `gorm:"not null;default:true"`
	CreatedAt    time.Time `gorm:"not null;default:current_timestamp"`
	UpdatedAt    time.Time `gorm:"not null;default:current_timestamp"`
}

func (User) TableName() string { return "users" }

type Client struct {
	ID               int64      `gorm:"primaryKey;autoIncrement"`
	Name             string     `gorm:"not null"`
	Document         string     `gorm:"not null;uniqueIndex"`
	BirthDate        *time.Time `gorm:"type:date"`
	Phone            string     `gorm:"not null"`
	SecondaryPhone   *string
	Street           string `gorm:"not null"`
	Neighborhood     string `gorm:"not null"`
	AddressNumber    string `gorm:"not null"`
	Complement       *string
	PostalCode       *string
	AddressType      string `gorm:"not null"`
	AddressReference *string
	Position         int       `gorm:"not null;default:0"`
	Active           bool      `gorm:"not null;default:true"`
	CreatedAt        time.Time `gorm:"not null;default:current_timestamp"`
	UpdatedAt        time.Time `gorm:"not null;default:current_timestamp"`
}

func (Client) TableName() string { return "clients" }

type Product struct {
	ID            int64           `gorm:"primaryKey;autoIncrement"`
	Name          string          `gorm:"not null"`
	PurchasePrice decimal.Decimal `gorm:"type:numeric(14,2);not null"`
	SalePrice     decimal.Decimal `gorm:"type:numeric(14,2);not null"`
	ImageURL      *string
	Active        bool      `gorm:"not null;default:true"`
	CreatedAt     time.Time `gorm:"not null;default:current_timestamp"`
	UpdatedAt     time.Time `gorm:"not null;default:current_timestamp"`
}

func (Product) TableName() string { return "products" }

type BillingPeriod struct {
	ID          int64     `gorm:"primaryKey;autoIncrement"`
	PeriodStart time.Time `gorm:"type:date;not null;uniqueIndex"`
	CreatedAt   time.Time `gorm:"not null;default:current_timestamp"`
}

func (BillingPeriod) TableName() string { return "billing_periods" }

type Invoice struct {
	ID                        int64  `gorm:"primaryKey;autoIncrement"`
	ClientID                  int64  `gorm:"not null"`
	BillingPeriodID           int64  `gorm:"not null"`
	Status                    string `gorm:"not null"`
	Observation               *string
	AccountBalanceBeforeIssue decimal.Decimal `gorm:"type:numeric(15,2);not null"`
	ProductsTotal             decimal.Decimal `gorm:"type:numeric(15,2);not null"`
	AccountBalanceAfterCharge decimal.Decimal `gorm:"type:numeric(15,2);not null"`
	CreatedAt                 time.Time       `gorm:"not null;default:current_timestamp"`
	UpdatedAt                 time.Time       `gorm:"not null;default:current_timestamp"`
	CanceledAt                *time.Time
	CancellationReason        *string
	Items                     []InvoiceItem `gorm:"foreignKey:InvoiceID"`
}

func (Invoice) TableName() string { return "invoices" }

type InvoiceItem struct {
	ID                int64           `gorm:"primaryKey;autoIncrement"`
	InvoiceID         int64           `gorm:"not null"`
	ProductID         int64           `gorm:"not null"`
	ProductName       string          `gorm:"not null"`
	Quantity          int             `gorm:"not null"`
	UnitPurchasePrice decimal.Decimal `gorm:"type:numeric(14,2);not null"`
	UnitSalePrice     decimal.Decimal `gorm:"type:numeric(14,2);not null"`
	PurchaseTotal     decimal.Decimal `gorm:"type:numeric(15,2);->"`
	SaleTotal         decimal.Decimal `gorm:"type:numeric(15,2);->"`
	ProfitTotal       decimal.Decimal `gorm:"type:numeric(15,2);->"`
	CreatedAt         time.Time       `gorm:"not null;default:current_timestamp"`
}

func (InvoiceItem) TableName() string { return "invoice_items" }

type Payment struct {
	ID             int64           `gorm:"primaryKey;autoIncrement"`
	ClientID       int64           `gorm:"not null"`
	Amount         decimal.Decimal `gorm:"type:numeric(15,2);not null"`
	EffectiveDate  time.Time       `gorm:"type:date;not null"`
	Observation    *string
	Status         string    `gorm:"not null"`
	CreatedAt      time.Time `gorm:"not null;default:current_timestamp"`
	ReversedAt     *time.Time
	ReversalReason *string
}

func (Payment) TableName() string { return "payments" }

type PaymentAllocation struct {
	ID             int64           `gorm:"primaryKey;autoIncrement"`
	ClientID       int64           `gorm:"not null"`
	PaymentID      int64           `gorm:"not null"`
	InvoiceID      int64           `gorm:"not null"`
	Amount         decimal.Decimal `gorm:"type:numeric(15,2);not null"`
	Status         string          `gorm:"not null"`
	AllocatedAt    time.Time       `gorm:"not null;default:current_timestamp"`
	ReversedAt     *time.Time
	ReversalKind   *string
	ReversalReason *string
}

func (PaymentAllocation) TableName() string { return "payment_allocations" }
