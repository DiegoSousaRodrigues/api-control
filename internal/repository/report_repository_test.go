package repository

import (
	"strings"
	"testing"
)

func TestClientBalanceQueryUsesHistoricalSnapshotsOnly(t *testing.T) {
	normalized := strings.ToLower(clientBalanceByMonthSQL)
	for _, required := range []string{
		`join order_sku`,
		`sum(os.price)`,
		`sum(os.purchase_total)`,
		`os.price - os.purchase_total`,
		`count(distinct o.id)`,
		`group by o.order_year, o.order_month`,
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("report query is missing %q", required)
		}
	}
	for _, forbidden := range []string{"join sku", "client_account_entry"} {
		if strings.Contains(normalized, forbidden) {
			t.Fatalf("report query must not use current product cost or account ledger: found %q", forbidden)
		}
	}
}

func TestClientBalanceQueryPreservesLegacyRowsAsIncomplete(t *testing.T) {
	normalized := strings.ToLower(clientBalanceByMonthSQL)
	for _, required := range []string{"snapshot_version <> 1", "purchase_total is null", "else null", "nulls last"} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("legacy handling is missing %q", required)
		}
	}
}
