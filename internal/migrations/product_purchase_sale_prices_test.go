package database

import (
	"os"
	"strings"
	"testing"
)

func TestProductPriceExpansionMigrationPreservesLegacyPrice(t *testing.T) {
	sql, err := os.ReadFile("000001_product_purchase_sale_prices.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	contents := string(sql)
	for _, required := range []string{
		"ADD COLUMN IF NOT EXISTS purchase_price NUMERIC(14,2)",
		"ADD COLUMN IF NOT EXISTS sale_price NUMERIC(14,2)",
		"SET sale_price = ROUND(price::numeric, 2)",
		"CHECK (sale_price > 0)",
		"ALTER COLUMN price TYPE NUMERIC(14,2)",
	} {
		if !strings.Contains(contents, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	if strings.Contains(contents, "DROP COLUMN price") {
		t.Fatal("expansion migration must preserve legacy sku.price")
	}
	if strings.Contains(contents, "ALTER COLUMN purchase_price SET NOT NULL") {
		t.Fatal("legacy purchase_price must remain nullable until business backfill")
	}
}
