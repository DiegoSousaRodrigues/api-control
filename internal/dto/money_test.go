package dto

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func TestMoneyAcceptsJSONNumberAndMarshalsWithoutQuotes(t *testing.T) {
	var value Money
	if err := json.Unmarshal([]byte("12.30"), &value); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if string(encoded) != "12.30" {
		t.Fatalf("JSON = %s, want numeric 12.30", encoded)
	}
}

func TestSkuProductRequestPriceRules(t *testing.T) {
	zero := NewMoney(decimal.Zero)
	positive := NewMoney(decimal.NewFromInt(1))
	negative := NewMoney(decimal.NewFromInt(-1))

	if err := (SkuProductRequest{Name: "Product", PurchasePrice: &zero, SalePrice: &positive}).Validate(); err != nil {
		t.Fatalf("zero purchase price must be valid: %v", err)
	}
	if err := (SkuProductRequest{Name: "Product", PurchasePrice: &negative, SalePrice: &positive}).Validate(); !errors.Is(err, ErrSkuPurchasePriceNegative) {
		t.Fatalf("negative purchase error = %v", err)
	}
	if err := (SkuProductRequest{Name: "Product", PurchasePrice: &zero, SalePrice: &zero}).Validate(); !errors.Is(err, ErrSkuSalePriceNotPositive) {
		t.Fatalf("zero sale error = %v", err)
	}
}

func TestMoneyRejectsStringsExtraScaleAndExponent(t *testing.T) {
	for _, input := range []string{`"12.30"`, "12.301", "1e2", "null"} {
		t.Run(input, func(t *testing.T) {
			var value Money
			if err := json.Unmarshal([]byte(input), &value); err == nil {
				t.Fatalf("Unmarshal(%s) error = nil", input)
			}
		})
	}
}

func TestSkuResponseMoneyFieldsAreNumbers(t *testing.T) {
	var purchase Money
	var sale Money
	if err := json.Unmarshal([]byte("5.25"), &purchase); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte("10.50"), &sale); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(SkuDTO{PurchasePrice: &purchase, SalePrice: sale})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"purchasePrice":"`) || strings.Contains(string(encoded), `"salePrice":"`) {
		t.Fatalf("money fields were encoded as strings: %s", encoded)
	}
}
