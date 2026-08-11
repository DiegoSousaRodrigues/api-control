package reportv2

import (
	"strings"
	"testing"
)

func TestClientBalanceQueryUsesOnlyIssuedHistoricalSnapshots(t *testing.T) {
	for _, required := range []string{"FROM invoices i", "LEFT JOIN invoice_items ii", "i.status = 'issued'",
		"SUM(ii.purchase_total)", "SUM(ii.sale_total)", "SUM(ii.profit_total)", "SUM(ii.quantity)",
		"sale_total <> products_total", "ORDER BY period_start DESC"} {
		if !strings.Contains(clientBalanceSQL, required) {
			t.Errorf("report query missing %q", required)
		}
	}
	for _, forbidden := range []string{" products ", "JOIN products", "payments", "payment_allocations", "client_account_entry"} {
		if strings.Contains(clientBalanceSQL, forbidden) {
			t.Errorf("report query must not use current product or financial data: %q", forbidden)
		}
	}
}

func TestClientBalanceQueryCountsInvoiceWithoutItemsAsMismatch(t *testing.T) {
	if !strings.Contains(clientBalanceSQL, "item_count = 0") {
		t.Fatal("invoice without items must fail snapshot reconciliation")
	}
}
