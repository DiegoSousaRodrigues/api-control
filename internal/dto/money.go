package dto

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

const moneyScale = 2

var (
	ErrMoneyMustBeNumber    = errors.New("money must be a JSON number")
	ErrMoneyTooManyDecimals = errors.New("money must have at most two decimal places")
	ErrMoneyOutOfRange      = errors.New("money is outside NUMERIC(14,2) range")
)

var maxMoney = decimal.RequireFromString("999999999999.99")

// Money is the JSON boundary for PostgreSQL NUMERIC(14,2). It deliberately
// emits a JSON number and never changes the process-wide decimal JSON setting.
type Money decimal.Decimal

func NewMoney(value decimal.Decimal) Money {
	return Money(value)
}

func (m Money) Decimal() decimal.Decimal {
	return decimal.Decimal(m)
}

func (m Money) MarshalJSON() ([]byte, error) {
	value := m.Decimal()
	if value.Abs().GreaterThan(maxMoney) {
		return nil, ErrMoneyOutOfRange
	}
	return []byte(value.StringFixed(moneyScale)), nil
}

func (m *Money) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) || data[0] == '"' {
		return ErrMoneyMustBeNumber
	}
	if bytes.ContainsAny(data, "eE") {
		return ErrMoneyMustBeNumber
	}

	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err != nil {
		return fmt.Errorf("%w: %v", ErrMoneyMustBeNumber, err)
	}

	value, err := decimal.NewFromString(number.String())
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMoneyMustBeNumber, err)
	}
	if value.Exponent() < -moneyScale {
		return ErrMoneyTooManyDecimals
	}
	if value.Abs().GreaterThan(maxMoney) {
		return ErrMoneyOutOfRange
	}

	*m = NewMoney(value)
	return nil
}
