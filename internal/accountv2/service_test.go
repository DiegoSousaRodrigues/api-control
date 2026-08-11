package accountv2

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"

	"github.com/shopspring/decimal"
)

type memoryAccount struct {
	mu           sync.Mutex
	clientID     int64
	invoiceTotal map[int64]decimal.Decimal
	paymentTotal map[int64]decimal.Decimal
	invoices     []OpenInvoice
	payments     []PaymentCredit
	allocations  []Allocation
	validateErr  error
}

type memoryUnitOfWork struct{ account *memoryAccount }

func (unit memoryUnitOfWork) WithLockedClient(ctx context.Context, clientID int64, callback func(AccountRepository) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	unit.account.mu.Lock()
	defer unit.account.mu.Unlock()
	if clientID != unit.account.clientID {
		return ErrInvalidClientID
	}
	before := append([]Allocation(nil), unit.account.allocations...)
	if err := callback(unit.account); err != nil {
		unit.account.allocations = before
		return err
	}
	return nil
}

func (account *memoryAccount) Position(ctx context.Context, clientID int64) (Position, error) {
	if err := ctx.Err(); err != nil {
		return Position{}, err
	}
	balance := decimal.Zero
	for _, total := range account.invoiceTotal {
		balance = balance.Add(total)
	}
	for _, total := range account.paymentTotal {
		balance = balance.Sub(total)
	}
	return NewPosition(clientID, balance), nil
}

func (account *memoryAccount) OpenInvoicesFIFO(ctx context.Context, clientID int64) ([]OpenInvoice, error) {
	rows := make([]OpenInvoice, 0, len(account.invoices))
	for _, invoice := range account.invoices {
		allocated := account.allocatedToInvoice(invoice.ID)
		invoice.PaidAmount = allocated
		invoice.OpenAmount = invoice.ProductsTotal.Sub(allocated)
		if invoice.OpenAmount.IsPositive() {
			rows = append(rows, invoice)
		}
	}
	return rows, ctx.Err()
}

func (account *memoryAccount) InvoiceOpenAmount(ctx context.Context, clientID, invoiceID int64) (OpenInvoice, error) {
	rows, err := account.OpenInvoicesFIFO(ctx, clientID)
	if err != nil {
		return OpenInvoice{}, err
	}
	for _, row := range rows {
		if row.ID == invoiceID {
			return row, nil
		}
	}
	return OpenInvoice{}, errors.New("invoice not found")
}

func (account *memoryAccount) AvailablePaymentsFIFO(ctx context.Context, clientID int64) ([]PaymentCredit, error) {
	rows := make([]PaymentCredit, 0, len(account.payments))
	for _, payment := range account.payments {
		allocated := account.allocatedFromPayment(payment.ID)
		payment.AllocatedAmount = allocated
		payment.UnallocatedAmount = payment.Amount.Sub(allocated)
		if payment.UnallocatedAmount.IsPositive() {
			rows = append(rows, payment)
		}
	}
	return rows, ctx.Err()
}

func (account *memoryAccount) PaymentCredit(ctx context.Context, clientID, paymentID int64) (PaymentCredit, error) {
	if err := ctx.Err(); err != nil {
		return PaymentCredit{}, err
	}
	for _, payment := range account.payments {
		if payment.ID == paymentID {
			allocated := account.allocatedFromPayment(payment.ID)
			payment.AllocatedAmount = allocated
			payment.UnallocatedAmount = payment.Amount.Sub(allocated)
			return payment, nil
		}
	}
	return PaymentCredit{}, errors.New("payment not found")
}

func (account *memoryAccount) CreateAllocation(ctx context.Context, clientID int64, allocation Allocation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if clientID != account.clientID || !validAmount(allocation.Amount) {
		return ErrInvalidAllocation
	}
	account.allocations = append(account.allocations, allocation)
	return nil
}

func (account *memoryAccount) ValidateConservation(context.Context, int64) error {
	if account.validateErr != nil {
		return account.validateErr
	}
	for id, total := range account.invoiceTotal {
		if account.allocatedToInvoice(id).GreaterThan(total) {
			return ErrInvoiceOverallocated
		}
	}
	for id, total := range account.paymentTotal {
		if account.allocatedFromPayment(id).GreaterThan(total) {
			return ErrPaymentOverallocated
		}
	}
	return nil
}

func (account *memoryAccount) allocatedToInvoice(id int64) decimal.Decimal {
	total := decimal.Zero
	for _, allocation := range account.allocations {
		if allocation.InvoiceID == id {
			total = total.Add(allocation.Amount)
		}
	}
	return total
}

func (account *memoryAccount) allocatedFromPayment(id int64) decimal.Decimal {
	total := decimal.Zero
	for _, allocation := range account.allocations {
		if allocation.PaymentID == id {
			total = total.Add(allocation.Amount)
		}
	}
	return total
}

func newMemoryAccount(invoiceTotals, paymentTotals map[int64]string) *memoryAccount {
	account := &memoryAccount{clientID: 1, invoiceTotal: make(map[int64]decimal.Decimal), paymentTotal: make(map[int64]decimal.Decimal)}
	for id, value := range invoiceTotals {
		total := decimal.RequireFromString(value)
		account.invoiceTotal[id] = total
		account.invoices = append(account.invoices, OpenInvoice{ID: id, ClientID: 1, ProductsTotal: total})
	}
	for id, value := range paymentTotals {
		total := decimal.RequireFromString(value)
		account.paymentTotal[id] = total
		account.payments = append(account.payments, PaymentCredit{ID: id, ClientID: 1, Amount: total})
	}
	sort.Slice(account.invoices, func(i, j int) bool { return account.invoices[i].ID < account.invoices[j].ID })
	sort.Slice(account.payments, func(i, j int) bool { return account.payments[i].ID < account.payments[j].ID })
	return account
}

func TestAllocatePaymentUsesInvoiceFIFOAndLeavesCredit(t *testing.T) {
	account := newMemoryAccount(map[int64]string{10: "30.00", 20: "50.00"}, map[int64]string{90: "100.00"})
	service, err := NewService(memoryUnitOfWork{account: account})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.AllocatePayment(context.Background(), 1, 90)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Allocations) != 2 || result.Allocations[0].InvoiceID != 10 || result.Allocations[1].InvoiceID != 20 {
		t.Fatalf("allocations = %#v", result.Allocations)
	}
	if !result.Allocations[0].Amount.Equal(decimal.NewFromInt(30)) || !result.Allocations[1].Amount.Equal(decimal.NewFromInt(50)) {
		t.Fatalf("allocation amounts = %#v", result.Allocations)
	}
	if result.Position.State != PositionCredit || !result.Position.CreditAmount.Equal(decimal.NewFromInt(20)) {
		t.Fatalf("position = %#v", result.Position)
	}
}

func TestAllocateInvoiceUsesPaymentFIFOAndAllowsPartialSettlement(t *testing.T) {
	account := newMemoryAccount(map[int64]string{10: "70.00"}, map[int64]string{80: "20.00", 90: "30.00"})
	service, _ := NewService(memoryUnitOfWork{account: account})
	result, err := service.AllocateInvoice(context.Background(), 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Allocations) != 2 || result.Allocations[0].PaymentID != 80 || result.Allocations[1].PaymentID != 90 {
		t.Fatalf("allocations = %#v", result.Allocations)
	}
	if result.Position.State != PositionDebt || !result.Position.DebtAmount.Equal(decimal.NewFromInt(20)) {
		t.Fatalf("position = %#v", result.Position)
	}
}

func TestInvariantFailureRollsBackWholeAllocation(t *testing.T) {
	account := newMemoryAccount(map[int64]string{10: "10.00"}, map[int64]string{90: "10.00"})
	account.validateErr = ErrInvalidActiveAllocation
	service, _ := NewService(memoryUnitOfWork{account: account})
	_, err := service.AllocatePayment(context.Background(), 1, 90)
	if !errors.Is(err, ErrInvalidActiveAllocation) {
		t.Fatalf("error = %v", err)
	}
	if len(account.allocations) != 0 {
		t.Fatalf("rollback retained allocations: %#v", account.allocations)
	}
}

func TestClientLockSerializesConcurrentAllocation(t *testing.T) {
	account := newMemoryAccount(map[int64]string{10: "80.00", 20: "80.00"}, map[int64]string{90: "100.00"})
	service, _ := NewService(memoryUnitOfWork{account: account})
	start := make(chan struct{})
	errorsFound := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, err := service.AllocatePayment(context.Background(), 1, 90)
			errorsFound <- err
		}()
	}
	close(start)
	var successes int
	for range 2 {
		if err := <-errorsFound; err == nil {
			successes++
		}
	}
	// The second operation observes the already consumed payment after acquiring
	// the same client lock. It is a successful no-op, matching paymentCreditSQL,
	// which returns posted payments even when unallocated_amount is zero.
	if successes != 2 {
		t.Fatalf("successes = %d, want 2", successes)
	}
	if got := account.allocatedFromPayment(90); !got.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("allocated = %s, want 100", got)
	}
	if got := account.allocatedToInvoice(10); !got.Equal(decimal.NewFromInt(80)) {
		t.Fatalf("first invoice allocated = %s, want 80", got)
	}
	if got := account.allocatedToInvoice(20); !got.Equal(decimal.NewFromInt(20)) {
		t.Fatalf("second invoice allocated = %s, want 20", got)
	}
}
