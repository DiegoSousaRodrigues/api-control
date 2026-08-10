package service

import (
	"errors"
	"testing"
	"time"
)

func TestValidateCompetenceRejectsFuture(t *testing.T) {
	originalNow := orderNow
	orderNow = func() time.Time { return time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { orderNow = originalNow })
	if err := validateCompetence(2026, 8); err != nil {
		t.Fatalf("current competence: %v", err)
	}
	if err := validateCompetence(2026, 9); !errors.Is(err, ErrOrderFutureCompetence) {
		t.Fatalf("future error = %v", err)
	}
}

func TestValidateCompetenceUsesSaoPauloAtUTCMonthBoundary(t *testing.T) {
	originalNow := orderNow
	// September in UTC, but still August in America/Sao_Paulo.
	orderNow = func() time.Time { return time.Date(2026, time.September, 1, 1, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { orderNow = originalNow })

	if err := validateCompetence(2026, 8); err != nil {
		t.Fatalf("August must still be current in Sao Paulo: %v", err)
	}
	if err := validateCompetence(2026, 9); !errors.Is(err, ErrOrderFutureCompetence) {
		t.Fatalf("September must still be future in Sao Paulo: %v", err)
	}
}
