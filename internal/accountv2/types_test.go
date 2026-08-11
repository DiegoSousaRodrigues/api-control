package accountv2

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestNewPositionClassifiesBalance(t *testing.T) {
	tests := []struct {
		name    string
		balance string
		state   string
		debt    string
		credit  string
	}{
		{name: "debt", balance: "125.30", state: PositionDebt, debt: "125.30", credit: "0"},
		{name: "settled", balance: "0", state: PositionSettled, debt: "0", credit: "0"},
		{name: "credit", balance: "-40.25", state: PositionCredit, debt: "0", credit: "40.25"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			position := NewPosition(7, decimal.RequireFromString(test.balance))
			if position.State != test.state || !position.DebtAmount.Equal(decimal.RequireFromString(test.debt)) ||
				!position.CreditAmount.Equal(decimal.RequireFromString(test.credit)) {
				t.Fatalf("position = %#v", position)
			}
		})
	}
}

func TestValidAmountHonorsNumericScaleAndRange(t *testing.T) {
	tests := []struct {
		amount string
		valid  bool
	}{
		{amount: "0.01", valid: true},
		{amount: "9999999999999.99", valid: true},
		{amount: "0", valid: false},
		{amount: "-1", valid: false},
		{amount: "1.001", valid: false},
		{amount: "10000000000000.00", valid: false},
	}
	for _, test := range tests {
		if got := validAmount(decimal.RequireFromString(test.amount)); got != test.valid {
			t.Errorf("validAmount(%s) = %v, want %v", test.amount, got, test.valid)
		}
	}
}
