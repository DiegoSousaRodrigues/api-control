package dto

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/api-control/internal/domain"
	"github.com/shopspring/decimal"
)

func TestOrderRequestRequiresNumericJSONFields(t *testing.T) {
	for _, payload := range []string{
		`{"clientId":"1","orderYear":2026,"orderMonth":8,"previousMonthPayment":0,"products":[]}`,
		`{"clientId":1,"orderYear":2026,"orderMonth":8,"previousMonthPayment":"0.00","products":[]}`,
	} {
		var request OrderRequestDTO
		if err := json.Unmarshal([]byte(payload), &request); err == nil {
			t.Fatalf("payload accepted: %s", payload)
		}
	}
}

func TestParseOrderSkuRequestRejectsEmptyProducts(t *testing.T) {
	_, err := ParseOrderSkuRequestToEntity(nil)
	if !errors.Is(err, ErrOrderRequiresProduct) {
		t.Fatalf("error = %v, want %v", err, ErrOrderRequiresProduct)
	}
}

func TestParseOrderSkuRequestRejectsNonPositiveQuantity(t *testing.T) {
	for _, quantity := range []int{0, -1} {
		t.Run("invalid", func(t *testing.T) {
			_, err := ParseOrderSkuRequestToEntity([]OrderSkuDTO{{ProductId: 1, Quantity: quantity}})
			if !errors.Is(err, ErrOrderProductQuantityPositive) {
				t.Fatalf("error = %v, want %v", err, ErrOrderProductQuantityPositive)
			}
		})
	}
}

func TestParseOrderSkuRequestRejectsDuplicatedProduct(t *testing.T) {
	_, err := ParseOrderSkuRequestToEntity([]OrderSkuDTO{
		{ProductId: 1, Quantity: 1},
		{ProductId: 1, Quantity: 2},
	})
	if !errors.Is(err, ErrOrderProductDuplicated) {
		t.Fatalf("error = %v, want %v", err, ErrOrderProductDuplicated)
	}
}

func TestParseOrderSkuRequestKeepsSkuAndQuantity(t *testing.T) {
	orderSkus, err := ParseOrderSkuRequestToEntity([]OrderSkuDTO{{ProductId: 7, Quantity: 3}})
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	if len(*orderSkus) != 1 {
		t.Fatalf("len = %d, want 1", len(*orderSkus))
	}

	orderSku := (*orderSkus)[0]
	if orderSku.SkuID != 7 || orderSku.Quantity != 3 {
		t.Fatalf("orderSku = %+v, want sku 7 quantity 3", orderSku)
	}
}

func TestParseOrderItemToDTOUsesHistoricalLineSnapshot(t *testing.T) {
	result := ParseOrderItemToDTO(domain.OrderSku{
		ID:       11,
		SkuID:    7,
		Name:     "Historical name",
		Price:    decimal.NewFromInt(30),
		Quantity: 3,
		Sku:      domain.Sku{ID: 7, Name: "Current name", SalePrice: decimal.NewFromInt(99)},
	})

	if result.ID != 11 || result.SkuID != 7 {
		t.Fatalf("ids = %+v, want order item 11 and sku 7", result)
	}
	if result.Name != "Historical name" {
		t.Fatalf("Name = %q, want Historical name", result.Name)
	}
	if result.Quantity != 3 {
		t.Fatalf("Quantity = %d, want 3", result.Quantity)
	}
	if !result.UnitPrice.Decimal().Equal(decimal.NewFromInt(10)) || !result.LineTotal.Decimal().Equal(decimal.NewFromInt(30)) {
		t.Fatalf("prices = unit %s total %s, want 10 and 30", result.UnitPrice.Decimal(), result.LineTotal.Decimal())
	}
	if result.Sku.Name != "Current name" {
		t.Fatalf("Sku.Name = %q, want Current name", result.Sku.Name)
	}
}
