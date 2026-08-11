package reportv2

import (
	"errors"

	"github.com/shopspring/decimal"
)

var maxReportAmount = decimal.RequireFromString("9999999999999.99")

type Amount decimal.Decimal

func newAmount(value decimal.Decimal) Amount { return Amount(value.Copy()) }

func (amount Amount) Decimal() decimal.Decimal { return decimal.Decimal(amount) }

func (amount Amount) MarshalJSON() ([]byte, error) {
	value := amount.Decimal()
	if value.Abs().GreaterThan(maxReportAmount) {
		return nil, errors.New("report amount is outside supported range")
	}
	return []byte(value.StringFixed(2)), nil
}

type ClientResponse struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type TotalsResponse struct {
	InvoiceCount  int64  `json:"invoiceCount"`
	QuantityTotal int64  `json:"quantityTotal"`
	PurchaseTotal Amount `json:"purchaseTotal"`
	SaleTotal     Amount `json:"saleTotal"`
	ProfitTotal   Amount `json:"profitTotal"`
}

type MonthResponse struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	TotalsResponse
}

type ClientBalanceResponse struct {
	Client ClientResponse  `json:"client"`
	Totals TotalsResponse  `json:"totals"`
	Months []MonthResponse `json:"months"`
}
