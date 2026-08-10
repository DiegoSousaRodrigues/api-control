package database

import (
	"os"
	"strings"
	"testing"
)

func TestOrderItemCostSnapshotMigrationIsExpansiveForLegacyRows(t *testing.T) {
	contents, err := os.ReadFile("000003_order_item_cost_snapshots.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(contents)
	for _, required := range []string{
		"snapshot_version SMALLINT NOT NULL DEFAULT 0",
		"unit_purchase_price NUMERIC(14,2)",
		"purchase_total NUMERIC(14,2)",
		"unit_sale_price NUMERIC(14,2)",
		"snapshot_version IN (0, 1)",
		"snapshot_version = 0",
		"purchase_total = unit_purchase_price * quantity",
		"price = unit_sale_price * quantity",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	if strings.Contains(sql, "UPDATE order_sku") || strings.Contains(sql, "JOIN sku") {
		t.Fatal("migration must not fabricate historical cost from current SKU data")
	}
	if migrationTransactionPattern.Match(contents) {
		t.Fatal("migration must delegate transaction control to the runner")
	}
}

func TestEmbeddedMigrationsIncludeOrderItemCostSnapshotsAfterLedger(t *testing.T) {
	items, err := loadMigrations(migrationFiles)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 3 {
		t.Fatalf("migration count = %d, want at least 3", len(items))
	}
	if items[0].version != 1 || items[1].version != 2 || items[2].version != 3 {
		t.Fatalf("versions = %d, %d, %d; want 1, 2, 3", items[0].version, items[1].version, items[2].version)
	}
	if items[2].id != "000003_order_item_cost_snapshots.up.sql" {
		t.Fatalf("third migration = %q", items[2].id)
	}
}
