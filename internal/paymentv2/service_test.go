package paymentv2

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/api-control/internal/accountv2"
	domainv2 "github.com/api-control/internal/domain/v2"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type memoryAllocation struct {
	accountv2.Allocation
	active bool
}

type paymentMemory struct {
	mu          sync.Mutex
	client      domainv2.Client
	invoices    map[int64]domainv2.Invoice
	periods     map[int64]time.Time
	payments    map[int64]domainv2.Payment
	allocations []memoryAllocation
	nextID      int64
}

type memoryUOW struct{ memory *paymentMemory }

func (unit memoryUOW) WithLockedClient(ctx context.Context, clientID int64, callback func(Repository, accountv2.AccountRepository) error) error {
	unit.memory.mu.Lock()
	defer unit.memory.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if clientID != unit.memory.client.ID {
		return gorm.ErrRecordNotFound
	}
	return callback(unit.memory, &memoryAccount{memory: unit.memory})
}

func newPaymentMemory() *paymentMemory {
	return &paymentMemory{client: domainv2.Client{ID: 1, Name: "Client", Active: true},
		invoices: make(map[int64]domainv2.Invoice), periods: make(map[int64]time.Time),
		payments: make(map[int64]domainv2.Payment), nextID: 100}
}

func newPaymentService(t *testing.T, memory *paymentMemory) *Service {
	t.Helper()
	service, err := NewService(memoryUOW{memory}, memory, func() time.Time {
		return time.Date(2026, time.November, 20, 12, 0, 0, 0, time.FixedZone("BRT", -3*60*60))
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func amount(value string) *accountv2.JSONAmount {
	result := accountv2.NewJSONAmount(decimal.RequireFromString(value))
	return &result
}

func (memory *paymentMemory) addInvoice(id int64, period time.Time, total string) {
	memory.invoices[id] = domainv2.Invoice{ID: id, ClientID: 1, Status: "issued", ProductsTotal: decimal.RequireFromString(total), CreatedAt: period}
	memory.periods[id] = period
}

func (memory *paymentMemory) FindClient(context.Context, int64) (*domainv2.Client, error) {
	client := memory.client
	return &client, nil
}

func (memory *paymentMemory) CreatePayment(_ context.Context, payment *domainv2.Payment) error {
	memory.nextID++
	payment.ID = memory.nextID
	payment.CreatedAt = time.Unix(memory.nextID, 0).UTC()
	memory.payments[payment.ID] = *payment
	return nil
}

func (memory *paymentMemory) FindPayment(_ context.Context, id int64) (*paymentRecord, error) {
	payment, exists := memory.payments[id]
	if !exists {
		return nil, gorm.ErrRecordNotFound
	}
	allocated := memory.allocatedFromPayment(id)
	return &paymentRecord{ID: id, ClientID: payment.ClientID, ClientName: memory.client.Name, ClientActive: memory.client.Active,
		Amount: payment.Amount, EffectiveDate: payment.EffectiveDate, Observation: payment.Observation, Status: payment.Status,
		AllocatedAmount: allocated, CreatedAt: payment.CreatedAt, ReversedAt: payment.ReversedAt, ReversalReason: payment.ReversalReason}, nil
}

func (memory *paymentMemory) FindActiveAllocations(_ context.Context, paymentID int64) ([]accountv2.Allocation, error) {
	rows := make([]accountv2.Allocation, 0)
	for _, allocation := range memory.allocations {
		if allocation.active && allocation.PaymentID == paymentID {
			rows = append(rows, allocation.Allocation)
		}
	}
	return rows, nil
}

func (memory *paymentMemory) ReversePayment(_ context.Context, paymentID int64, at time.Time, reason string) error {
	payment := memory.payments[paymentID]
	payment.Status, payment.ReversedAt, payment.ReversalReason = "reversed", &at, &reason
	memory.payments[paymentID] = payment
	return nil
}

func (memory *paymentMemory) ReverseActiveAllocations(_ context.Context, paymentID int64, _ time.Time, _ string) error {
	for index := range memory.allocations {
		if memory.allocations[index].PaymentID == paymentID {
			memory.allocations[index].active = false
		}
	}
	return nil
}

func (memory *paymentMemory) ReverseOtherActiveAllocations(_ context.Context, _ int64, paymentID int64, _ time.Time, _ string) error {
	for index := range memory.allocations {
		if memory.allocations[index].PaymentID != paymentID {
			memory.allocations[index].active = false
		}
	}
	return nil
}

func (memory *paymentMemory) FindPaymentClientID(_ context.Context, id int64) (int64, error) {
	payment, exists := memory.payments[id]
	if !exists {
		return 0, gorm.ErrRecordNotFound
	}
	return payment.ClientID, nil
}

func (memory *paymentMemory) ListPayments(context.Context, listQuery) ([]paymentRecord, error) {
	return nil, nil
}

type memoryAccount struct{ memory *paymentMemory }

func (repository *memoryAccount) Position(_ context.Context, clientID int64) (accountv2.Position, error) {
	balance := decimal.Zero
	for _, invoice := range repository.memory.invoices {
		if invoice.Status == "issued" {
			balance = balance.Add(invoice.ProductsTotal)
		}
	}
	for _, payment := range repository.memory.payments {
		if payment.Status == "posted" {
			balance = balance.Sub(payment.Amount)
		}
	}
	return accountv2.NewPosition(clientID, balance), nil
}

func (repository *memoryAccount) OpenInvoicesFIFO(context.Context, int64) ([]accountv2.OpenInvoice, error) {
	rows := make([]accountv2.OpenInvoice, 0)
	for _, invoice := range repository.memory.invoices {
		if invoice.Status != "issued" {
			continue
		}
		open := invoice.ProductsTotal.Sub(repository.memory.allocatedToInvoice(invoice.ID))
		if open.IsPositive() {
			rows = append(rows, accountv2.OpenInvoice{ID: invoice.ID, ClientID: invoice.ClientID,
				PeriodStart: repository.memory.periods[invoice.ID], CreatedAt: invoice.CreatedAt,
				ProductsTotal: invoice.ProductsTotal, OpenAmount: open})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].PeriodStart.Before(rows[j].PeriodStart) })
	return rows, nil
}

func (repository *memoryAccount) InvoiceOpenAmount(context.Context, int64, int64) (accountv2.OpenInvoice, error) {
	return accountv2.OpenInvoice{}, gorm.ErrRecordNotFound
}

func (repository *memoryAccount) AvailablePaymentsFIFO(_ context.Context, clientID int64) ([]accountv2.PaymentCredit, error) {
	rows := make([]accountv2.PaymentCredit, 0)
	for _, payment := range repository.memory.payments {
		if payment.Status != "posted" {
			continue
		}
		unallocated := payment.Amount.Sub(repository.memory.allocatedFromPayment(payment.ID))
		if unallocated.IsPositive() {
			rows = append(rows, accountv2.PaymentCredit{ID: payment.ID, ClientID: clientID, Amount: payment.Amount,
				EffectiveDate: payment.EffectiveDate, CreatedAt: payment.CreatedAt, UnallocatedAmount: unallocated})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].EffectiveDate.Equal(rows[j].EffectiveDate) {
			return rows[i].ID < rows[j].ID
		}
		return rows[i].EffectiveDate.Before(rows[j].EffectiveDate)
	})
	return rows, nil
}

func (repository *memoryAccount) PaymentCredit(_ context.Context, clientID, paymentID int64) (accountv2.PaymentCredit, error) {
	payment, exists := repository.memory.payments[paymentID]
	if !exists || payment.Status != "posted" {
		return accountv2.PaymentCredit{}, gorm.ErrRecordNotFound
	}
	allocated := repository.memory.allocatedFromPayment(paymentID)
	return accountv2.PaymentCredit{ID: paymentID, ClientID: clientID, Amount: payment.Amount,
		EffectiveDate: payment.EffectiveDate, CreatedAt: payment.CreatedAt, AllocatedAmount: allocated,
		UnallocatedAmount: payment.Amount.Sub(allocated)}, nil
}

func (repository *memoryAccount) CreateAllocation(_ context.Context, _ int64, allocation accountv2.Allocation) error {
	repository.memory.allocations = append(repository.memory.allocations, memoryAllocation{Allocation: allocation, active: true})
	return nil
}

func (repository *memoryAccount) ValidateConservation(context.Context, int64) error { return nil }

func (memory *paymentMemory) allocatedFromPayment(id int64) decimal.Decimal {
	total := decimal.Zero
	for _, allocation := range memory.allocations {
		if allocation.active && allocation.PaymentID == id {
			total = total.Add(allocation.Amount)
		}
	}
	return total
}

func (memory *paymentMemory) allocatedToInvoice(id int64) decimal.Decimal {
	total := decimal.Zero
	for _, allocation := range memory.allocations {
		if allocation.active && allocation.InvoiceID == id {
			total = total.Add(allocation.Amount)
		}
	}
	return total
}

func TestMainScenarioPartialMultipleAndCreditOnCredit(t *testing.T) {
	memory := newPaymentMemory()
	memory.addInvoice(1, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), "500")
	memory.addInvoice(2, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), "500")
	memory.payments[50] = domainv2.Payment{ID: 50, ClientID: 1, Amount: decimal.NewFromInt(250),
		EffectiveDate: time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC), Status: "posted"}
	memory.allocations = append(memory.allocations, memoryAllocation{Allocation: accountv2.Allocation{PaymentID: 50, InvoiceID: 1, Amount: decimal.NewFromInt(250)}, active: true})
	service := newPaymentService(t, memory)

	first, err := service.Create(context.Background(), CreateRequest{ClientID: 1, Amount: amount("850"), EffectiveDate: "2026-10-15"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Account.Position != accountv2.PositionCredit || !first.Account.CreditAmount.Decimal().Equal(decimal.NewFromInt(100)) ||
		!first.AllocatedAmount.Decimal().Equal(decimal.NewFromInt(750)) || len(first.Allocations) != 2 {
		t.Fatalf("first payment = %#v", first)
	}
	second, err := service.Create(context.Background(), CreateRequest{ClientID: 1, Amount: amount("200"), EffectiveDate: "2026-10-20"})
	if err != nil {
		t.Fatal(err)
	}
	if second.Account.Position != accountv2.PositionCredit || !second.Account.CreditAmount.Decimal().Equal(decimal.NewFromInt(300)) ||
		!second.CreditAmount.Decimal().Equal(decimal.NewFromInt(200)) {
		t.Fatalf("credit-on-credit payment = %#v", second)
	}
}

func TestRetroactivePaymentSettlesCurrentlyOpenInvoice(t *testing.T) {
	memory := newPaymentMemory()
	memory.addInvoice(1, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), "500")
	service := newPaymentService(t, memory)
	response, err := service.Create(context.Background(), CreateRequest{ClientID: 1, Amount: amount("200"), EffectiveDate: "2026-01-01"})
	if err != nil {
		t.Fatal(err)
	}
	if !response.AllocatedAmount.Decimal().Equal(decimal.NewFromInt(200)) || response.Account.Position != accountv2.PositionDebt {
		t.Fatalf("retroactive response = %#v", response)
	}
}

func TestReversalReallocatesOtherPostedPaymentsFIFO(t *testing.T) {
	memory := newPaymentMemory()
	memory.addInvoice(1, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), "500")
	memory.addInvoice(2, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), "500")
	memory.payments[10] = domainv2.Payment{ID: 10, ClientID: 1, Amount: decimal.NewFromInt(600), EffectiveDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), Status: "posted"}
	memory.payments[20] = domainv2.Payment{ID: 20, ClientID: 1, Amount: decimal.NewFromInt(600), EffectiveDate: time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC), Status: "posted"}
	memory.allocations = []memoryAllocation{
		{Allocation: accountv2.Allocation{PaymentID: 10, InvoiceID: 1, Amount: decimal.NewFromInt(500)}, active: true},
		{Allocation: accountv2.Allocation{PaymentID: 10, InvoiceID: 2, Amount: decimal.NewFromInt(100)}, active: true},
		{Allocation: accountv2.Allocation{PaymentID: 20, InvoiceID: 2, Amount: decimal.NewFromInt(400)}, active: true},
	}
	service := newPaymentService(t, memory)
	response, err := service.Reverse(context.Background(), 10, ReverseRequest{Reason: "duplicate"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != "reversed" || response.Account.Position != accountv2.PositionDebt ||
		!response.Account.DebtAmount.Decimal().Equal(decimal.NewFromInt(400)) {
		t.Fatalf("reversal response = %#v", response)
	}
	if !memory.allocatedToInvoice(1).Equal(decimal.NewFromInt(500)) || !memory.allocatedToInvoice(2).Equal(decimal.NewFromInt(100)) {
		t.Fatalf("reallocated invoice1=%s invoice2=%s", memory.allocatedToInvoice(1), memory.allocatedToInvoice(2))
	}
}

func TestListAllowsMaximum366DayRange(t *testing.T) {
	memory := newPaymentMemory()
	service := newPaymentService(t, memory)
	dateFrom := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	dateTo := dateFrom.Add(366 * 24 * time.Hour)
	if _, err := service.List(context.Background(), ListFilter{DateFrom: &dateFrom, DateTo: &dateTo}); err != nil {
		t.Fatalf("366-day range must be accepted: %v", err)
	}
	tooLate := dateTo.Add(24 * time.Hour)
	if _, err := service.List(context.Background(), ListFilter{DateFrom: &dateFrom, DateTo: &tooLate}); !errors.Is(err, ErrInvalidFilter) {
		t.Fatalf("range above 366 days error = %v", err)
	}
}

func TestConcurrentPaymentsSerializeWithoutOverallocating(t *testing.T) {
	memory := newPaymentMemory()
	memory.addInvoice(1, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), "100")
	service := newPaymentService(t, memory)
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, err := service.Create(context.Background(), CreateRequest{ClientID: 1, Amount: amount("100"), EffectiveDate: "2026-10-01"})
			results <- err
		}()
	}
	close(start)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if !memory.allocatedToInvoice(1).Equal(decimal.NewFromInt(100)) {
		t.Fatalf("invoice allocated = %s", memory.allocatedToInvoice(1))
	}
	position, _ := (&memoryAccount{memory}).Position(context.Background(), 1)
	if position.State != accountv2.PositionCredit || !position.CreditAmount.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("position = %#v", position)
	}
}

func TestFuturePaymentAndSecondReversalAreRejected(t *testing.T) {
	memory := newPaymentMemory()
	service := newPaymentService(t, memory)
	_, err := service.Create(context.Background(), CreateRequest{ClientID: 1, Amount: amount("1"), EffectiveDate: "2026-11-21"})
	if !errors.Is(err, ErrFutureEffectiveDate) {
		t.Fatalf("future error = %v", err)
	}
	memory.payments[10] = domainv2.Payment{ID: 10, ClientID: 1, Amount: decimal.NewFromInt(1), Status: "reversed"}
	_, err = service.Reverse(context.Background(), 10, ReverseRequest{Reason: "again"})
	if !errors.Is(err, ErrPaymentNotPosted) {
		t.Fatalf("second reversal error = %v", err)
	}
}
