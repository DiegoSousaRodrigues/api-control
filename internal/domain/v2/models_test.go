package v2

import "testing"

func TestModelsTargetOnlyPluralV2Tables(t *testing.T) {
	tables := map[string]string{
		"user":               (User{}).TableName(),
		"client":             (Client{}).TableName(),
		"product":            (Product{}).TableName(),
		"billing period":     (BillingPeriod{}).TableName(),
		"invoice":            (Invoice{}).TableName(),
		"invoice item":       (InvoiceItem{}).TableName(),
		"payment":            (Payment{}).TableName(),
		"payment allocation": (PaymentAllocation{}).TableName(),
	}
	want := map[string]string{
		"user": "users", "client": "clients", "product": "products",
		"billing period": "billing_periods", "invoice": "invoices",
		"invoice item": "invoice_items", "payment": "payments",
		"payment allocation": "payment_allocations",
	}
	for model, table := range tables {
		if table != want[model] {
			t.Fatalf("%s table = %q, want %q", model, table, want[model])
		}
	}
}
