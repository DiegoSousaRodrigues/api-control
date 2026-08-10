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

func TestOrderUpdateFieldsDoesNotPersistAssociations(t *testing.T) {
	fields := orderUpdateFields(domain.Order{
		ID:          99,
		ClientId:    2,
		Observation: "Updated",
		OrderSkus:   []domain.OrderSku{{ID: 1}},
	})

	assertHasFields(t, fields, []string{"last_updated", "client_id", "observation"})
	assertMissingFields(t, fields, []string{"id", "date_created", "deleted_at", "client", "order_skus"})
}

func TestApplyOrderSkuSnapshotUsesSkuDataAndQuantity(t *testing.T) {
	orderSku := domain.OrderSku{Quantity: 3}
	sku := domain.Sku{Name: "Product", SalePrice: decimal.RequireFromString("12.50")}

	applyOrderSkuSnapshot(&orderSku, sku)

	if orderSku.Name != "Product" {
		t.Fatalf("Name = %q, want Product", orderSku.Name)
	}
	if !orderSku.Price.Equal(decimal.RequireFromString("37.50")) {
		t.Fatalf("Price = %v, want 37.5", orderSku.Price)
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
