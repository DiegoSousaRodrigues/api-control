package dto

import (
	"errors"
	"testing"

	"github.com/api-control/internal/domain"
)

func TestParseOrderSkuRequestRejectsEmptyProducts(t *testing.T) {
	_, err := ParseOrderSkuRequestToEntity(nil)
	if !errors.Is(err, ErrOrderRequiresProduct) {
		t.Fatalf("error = %v, want %v", err, ErrOrderRequiresProduct)
	}
}

func TestParseOrderSkuRequestRejectsNonPositiveQuantity(t *testing.T) {
	for _, quantity := range []string{"0", "-1"} {
		t.Run(quantity, func(t *testing.T) {
			_, err := ParseOrderSkuRequestToEntity([]OrderSkuDTO{{ProductId: "1", Quantity: quantity}})
			if !errors.Is(err, ErrOrderProductQuantityPositive) {
				t.Fatalf("error = %v, want %v", err, ErrOrderProductQuantityPositive)
			}
		})
	}
}

func TestParseOrderSkuRequestRejectsDuplicatedProduct(t *testing.T) {
	_, err := ParseOrderSkuRequestToEntity([]OrderSkuDTO{
		{ProductId: "1", Quantity: "1"},
		{ProductId: "1", Quantity: "2"},
	})
	if !errors.Is(err, ErrOrderProductDuplicated) {
		t.Fatalf("error = %v, want %v", err, ErrOrderProductDuplicated)
	}
}

func TestParseOrderSkuRequestKeepsSkuAndQuantity(t *testing.T) {
	orderSkus, err := ParseOrderSkuRequestToEntity([]OrderSkuDTO{{ProductId: "7", Quantity: "3"}})
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
		Price:    30,
		Quantity: 3,
		Sku:      domain.Sku{ID: 7, Name: "Current name", Price: 99},
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
	if result.UnitPrice == "" || result.LineTotal == "" {
		t.Fatalf("prices must be formatted, got unit=%q total=%q", result.UnitPrice, result.LineTotal)
	}
	if result.Sku.Name != "Current name" {
		t.Fatalf("Sku.Name = %q, want Current name", result.Sku.Name)
	}
}
