package repository

import (
	"testing"

	"github.com/api-control/internal/domain"
	"github.com/shopspring/decimal"
)

func TestClientUpdateFieldsUsesAllowlist(t *testing.T) {
	fields := clientUpdateFields(domain.Client{
		ID:       99,
		Active:   true,
		Name:     "Client",
		Document: "123",
		Position: 7,
	})

	assertHasFields(t, fields, []string{
		"last_updated",
		"name",
		"document",
		"phone",
		"telephone",
		"birthdate",
		"street",
		"quarter",
		"number",
		"complement",
		"zipcode",
		"address_type",
		"address_reference",
		"position",
	})
	assertMissingFields(t, fields, []string{"id", "date_created", "active", "orders"})
}

func TestSkuUpdateFieldsPreservesActiveAndMissingImage(t *testing.T) {
	fields := skuUpdateFields(domain.Sku{
		ID:        99,
		Active:    true,
		Name:      "Product",
		Price:     10.5,
		SalePrice: decimal.RequireFromString("10.50"),
	})

	assertHasFields(t, fields, []string{"last_updated", "name", "price", "purchase_price", "sale_price"})
	assertMissingFields(t, fields, []string{"id", "date_created", "active", "image_url", "order_skus"})
}

func TestSkuUpdateFieldsIncludesImageOnlyWhenProvided(t *testing.T) {
	imageURL := "https://example.com/image.png"
	fields := skuUpdateFields(domain.Sku{ImageUrl: &imageURL})

	if fields["image_url"] != &imageURL {
		t.Fatalf("image_url = %v, want provided image URL pointer", fields["image_url"])
	}
}

func TestApplyOrderSkuSnapshotUsesSkuDataAndQuantity(t *testing.T) {
	orderSku := domain.OrderSku{Quantity: 3}
	purchasePrice := decimal.RequireFromString("7.25")
	sku := domain.Sku{Name: "Product", PurchasePrice: &purchasePrice, SalePrice: decimal.RequireFromString("12.50")}

	if err := applyOrderSkuSnapshot(&orderSku, sku); err != nil {
		t.Fatal(err)
	}

	if orderSku.Name != "Product" {
		t.Fatalf("Name = %q, want Product", orderSku.Name)
	}
	if !orderSku.Price.Equal(decimal.RequireFromString("37.50")) {
		t.Fatalf("Price = %v, want 37.5", orderSku.Price)
	}
	if orderSku.SnapshotVersion != 1 {
		t.Fatalf("SnapshotVersion = %d, want 1", orderSku.SnapshotVersion)
	}
	if orderSku.UnitPurchasePrice == nil || !orderSku.UnitPurchasePrice.Equal(decimal.RequireFromString("7.25")) {
		t.Fatalf("UnitPurchasePrice = %v, want 7.25", orderSku.UnitPurchasePrice)
	}
	if orderSku.PurchaseTotal == nil || !orderSku.PurchaseTotal.Equal(decimal.RequireFromString("21.75")) {
		t.Fatalf("PurchaseTotal = %v, want 21.75", orderSku.PurchaseTotal)
	}
	if orderSku.UnitSalePrice == nil || !orderSku.UnitSalePrice.Equal(decimal.RequireFromString("12.50")) {
		t.Fatalf("UnitSalePrice = %v, want 12.50", orderSku.UnitSalePrice)
	}
	if orderSku.UnitPurchasePrice == sku.PurchasePrice {
		t.Fatal("UnitPurchasePrice must be an independent decimal pointer")
	}

	purchasePrice = decimal.RequireFromString("99.99")
	sku.SalePrice = decimal.RequireFromString("88.88")
	if !orderSku.UnitPurchasePrice.Equal(decimal.RequireFromString("7.25")) || !orderSku.UnitSalePrice.Equal(decimal.RequireFromString("12.50")) {
		t.Fatal("changing SKU prices must not mutate the order snapshot")
	}
}

func assertHasFields(t *testing.T, fields map[string]interface{}, names []string) {
	t.Helper()

	for _, name := range names {
		if _, ok := fields[name]; !ok {
			t.Fatalf("field %q missing from update allowlist", name)
		}
	}
}

func assertMissingFields(t *testing.T, fields map[string]interface{}, names []string) {
	t.Helper()

	for _, name := range names {
		if _, ok := fields[name]; ok {
			t.Fatalf("field %q should not be updated", name)
		}
	}
}
