package database

import (
	"os"
	"strings"
	"testing"
)

func TestOrderCompetenceLedgerMigrationKeepsLegacyCompetenceNullable(t *testing.T) {
	sql, err := os.ReadFile("000002_order_competence_ledger.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(sql)
	for _, required := range []string{"order_year SMALLINT", "order_month SMALLINT", "CREATE UNIQUE INDEX uq_order_client_competence", "CREATE TABLE client_account_entry", "NUMERIC(14,2)"} {
		if !strings.Contains(text, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	if !strings.Contains(text, "ADD COLUMN order_year SMALLINT") && !strings.Contains(text, "ADD COLUMN order_month SMALLINT") {
		t.Fatal("order competence columns must be added nullable for legacy rows")
	}
}
