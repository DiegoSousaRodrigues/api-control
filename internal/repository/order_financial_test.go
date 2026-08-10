package repository

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/api-control/internal/domain"
	"github.com/shopspring/decimal"
)

func TestPreviousCompetenceHandlesJanuary(t *testing.T) {
	year, month := previousCompetence(2026, 1)
	if year != 2025 || month != 12 {
		t.Fatalf("previous = %d/%d", month, year)
	}
}

func TestOrderAddRejectsMissingCompetenceWithoutOpeningDatabase(t *testing.T) {
	err := (&orderRepository{}).Add(domain.Order{})
	if !errors.Is(err, ErrOrderCompetenceRequired) {
		t.Fatalf("error = %v, want %v", err, ErrOrderCompetenceRequired)
	}
}

func TestCalculateCarriedBalanceRejectsOverpayment(t *testing.T) {
	_, err := calculateCarriedBalance(decimal.NewFromInt(10), decimal.NewFromInt(11))
	if !errors.Is(err, ErrOrderPaymentExceedsBalance) {
		t.Fatalf("error = %v", err)
	}
	balance, err := calculateCarriedBalance(decimal.NewFromInt(10), decimal.NewFromInt(4))
	if err != nil || !balance.Equal(decimal.NewFromInt(6)) {
		t.Fatalf("balance = %s error = %v", balance, err)
	}
}

func TestOrderCreateGuardsAgainstRetroactiveSnapshots(t *testing.T) {
	source, err := os.ReadFile("order_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if !strings.Contains(text, "order_year > ?") || !strings.Contains(text, "ErrOrderRetroactiveCompetence") {
		t.Fatal("missing retroactive guard")
	}
}
