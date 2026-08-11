package invoicev2

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

type invoiceMemory struct {
	mu          sync.Mutex
	client      domainv2.Client
	products    map[int64]domainv2.Product
	periods     map[string]domainv2.BillingPeriod
	invoices    map[int64]domainv2.Invoice
	items       map[int64][]domainv2.InvoiceItem
	payments    map[int64]domainv2.Payment
	allocations []memoryAllocation
	nextID      int64
}

type memoryUOW struct{ memory *invoiceMemory }

func (unit memoryUOW) WithLockedClient(ctx context.Context, clientID int64, callback func(Repository, accountv2.AccountRepository) error) error {
	unit.memory.mu.Lock()
	defer unit.memory.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if clientID != unit.memory.client.ID {
		return gorm.ErrRecordNotFound
	}
	return callback(unit.memory, &memoryAccountRepository{memory: unit.memory})
}

func newInvoiceMemory() *invoiceMemory {
	return &invoiceMemory{
		client: domainv2.Client{ID: 1, Name: "Client", Active: true},
		products: map[int64]domainv2.Product{
			10: {ID: 10, Name: "Loss item", PurchasePrice: decimal.NewFromInt(600), SalePrice: decimal.NewFromInt(500), Active: true},
			20: {ID: 20, Name: "Regular item", PurchasePrice: decimal.NewFromInt(100), SalePrice: decimal.NewFromInt(200), Active: true},
		},
		periods: make(map[string]domainv2.BillingPeriod), invoices: make(map[int64]domainv2.Invoice),
		items: make(map[int64][]domainv2.InvoiceItem), payments: make(map[int64]domainv2.Payment), nextID: 100,
	}
}

func newInvoiceService(t *testing.T, memory *invoiceMemory) *Service {
	t.Helper()
	service, err := NewService(memoryUOW{memory: memory}, memory, func() time.Time {
		return time.Date(2026, time.November, 15, 12, 0, 0, 0, time.FixedZone("BRT", -3*60*60))
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func (memory *invoiceMemory) FindClient(context.Context, int64) (*domainv2.Client, error) {
	client := memory.client
	return &client, nil
}

func (memory *invoiceMemory) FindOrCreatePeriod(_ context.Context, period time.Time) (*domainv2.BillingPeriod, error) {
	key := period.Format("2006-01-02")
	value, exists := memory.periods[key]
	if !exists {
		memory.nextID++
		value = domainv2.BillingPeriod{ID: memory.nextID, PeriodStart: period}
		memory.periods[key] = value
	}
	return &value, nil
}

func (memory *invoiceMemory) HasActiveInvoiceInPeriod(_ context.Context, clientID, periodID int64) (bool, error) {
	for _, invoice := range memory.invoices {
		if invoice.ClientID == clientID && invoice.BillingPeriodID == periodID && invoice.Status == "issued" {
			return true, nil
		}
	}
	return false, nil
}

func (memory *invoiceMemory) HasLaterActiveInvoice(_ context.Context, clientID int64, period time.Time) (bool, error) {
	for _, invoice := range memory.invoices {
		billingPeriod := memory.periodByID(invoice.BillingPeriodID)
		if invoice.ClientID == clientID && invoice.Status == "issued" && billingPeriod.PeriodStart.After(period) {
			return true, nil
		}
	}
	return false, nil
}

func (memory *invoiceMemory) FindActiveProducts(_ context.Context, ids []int64) ([]domainv2.Product, error) {
	products := make([]domainv2.Product, 0, len(ids))
	for _, id := range ids {
		if product, exists := memory.products[id]; exists && product.Active {
			products = append(products, product)
		}
	}
	return products, nil
}

func (memory *invoiceMemory) CreateInvoice(_ context.Context, invoice *domainv2.Invoice) error {
	memory.nextID++
	invoice.ID = memory.nextID
	invoice.CreatedAt = time.Unix(memory.nextID, 0).UTC()
	memory.invoices[invoice.ID] = *invoice
	return nil
}

func (memory *invoiceMemory) CreateItems(_ context.Context, items []domainv2.InvoiceItem) error {
	for index := range items {
		memory.nextID++
		items[index].ID = memory.nextID
		quantity := decimal.NewFromInt(int64(items[index].Quantity))
		items[index].PurchaseTotal = items[index].UnitPurchasePrice.Mul(quantity)
		items[index].SaleTotal = items[index].UnitSalePrice.Mul(quantity)
		items[index].ProfitTotal = items[index].UnitSalePrice.Sub(items[index].UnitPurchasePrice).Mul(quantity)
		memory.items[items[index].InvoiceID] = append(memory.items[items[index].InvoiceID], items[index])
	}
	return nil
}

func (memory *invoiceMemory) PersistedSaleTotal(_ context.Context, invoiceID int64) (decimal.Decimal, error) {
	total := decimal.Zero
	for _, item := range memory.items[invoiceID] {
		total = total.Add(item.SaleTotal)
	}
	return total, nil
}

func (memory *invoiceMemory) FindInvoice(_ context.Context, id int64) (*invoiceRecord, error) {
	invoice, exists := memory.invoices[id]
	if !exists {
		return nil, gorm.ErrRecordNotFound
	}
	period := memory.periodByID(invoice.BillingPeriodID)
	paid := memory.allocatedToInvoice(id)
	open := invoice.ProductsTotal.Sub(paid)
	if invoice.Status == "canceled" {
		paid, open = decimal.Zero, decimal.Zero
	}
	return &invoiceRecord{ID: id, Status: invoice.Status, PeriodStart: period.PeriodStart,
		ClientID: invoice.ClientID, ClientName: memory.client.Name, ClientActive: memory.client.Active,
		ProductsTotal: invoice.ProductsTotal, AccountBalanceBeforeIssue: invoice.AccountBalanceBeforeIssue,
		AccountBalanceAfterCharge: invoice.AccountBalanceAfterCharge, PaidAmount: paid, OpenAmount: open,
		Observation: invoice.Observation, CreatedAt: invoice.CreatedAt, CanceledAt: invoice.CanceledAt,
		CancellationReason: invoice.CancellationReason}, nil
}

func (memory *invoiceMemory) FindItems(_ context.Context, invoiceID int64) ([]domainv2.InvoiceItem, error) {
	return append([]domainv2.InvoiceItem(nil), memory.items[invoiceID]...), nil
}

func (memory *invoiceMemory) LatestActiveInvoiceID(context.Context, int64) (int64, error) {
	var latest domainv2.Invoice
	var latestPeriod time.Time
	for _, invoice := range memory.invoices {
		if invoice.Status != "issued" {
			continue
		}
		period := memory.periodByID(invoice.BillingPeriodID).PeriodStart
		if latest.ID == 0 || period.After(latestPeriod) || (period.Equal(latestPeriod) && invoice.ID > latest.ID) {
			latest, latestPeriod = invoice, period
		}
	}
	if latest.ID == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	return latest.ID, nil
}

func (memory *invoiceMemory) CancelInvoice(_ context.Context, invoiceID int64, at time.Time, reason string) error {
	invoice := memory.invoices[invoiceID]
	invoice.Status, invoice.CanceledAt, invoice.CancellationReason = "canceled", &at, &reason
	memory.invoices[invoiceID] = invoice
	return nil
}

func (memory *invoiceMemory) ReverseActiveAllocations(_ context.Context, invoiceID int64, _ time.Time, _ string) error {
	for index := range memory.allocations {
		if memory.allocations[index].InvoiceID == invoiceID {
			memory.allocations[index].active = false
		}
	}
	return nil
}

func (memory *invoiceMemory) FindInvoiceClientID(_ context.Context, invoiceID int64) (int64, error) {
	invoice, exists := memory.invoices[invoiceID]
	if !exists {
		return 0, gorm.ErrRecordNotFound
	}
	return invoice.ClientID, nil
}

func (memory *invoiceMemory) ListInvoices(_ context.Context, query listQuery) ([]invoiceRecord, error) {
	rows := make([]invoiceRecord, 0)
	for id := range memory.invoices {
		record, _ := memory.FindInvoice(context.Background(), id)
		if record.PeriodStart.Equal(query.PeriodStart) && (query.ClientID == nil || record.ClientID == *query.ClientID) {
			rows = append(rows, *record)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID > rows[j].ID })
	if len(rows) > query.Limit {
		rows = rows[:query.Limit]
	}
	return rows, nil
}

func (memory *invoiceMemory) periodByID(id int64) domainv2.BillingPeriod {
	for _, period := range memory.periods {
		if period.ID == id {
			return period
		}
	}
	return domainv2.BillingPeriod{}
}

type memoryAccountRepository struct{ memory *invoiceMemory }

func (repository *memoryAccountRepository) Position(_ context.Context, clientID int64) (accountv2.Position, error) {
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

func (repository *memoryAccountRepository) OpenInvoicesFIFO(context.Context, int64) ([]accountv2.OpenInvoice, error) {
	rows := make([]accountv2.OpenInvoice, 0)
	for _, invoice := range repository.memory.invoices {
		if invoice.Status != "issued" {
			continue
		}
		open := invoice.ProductsTotal.Sub(repository.memory.allocatedToInvoice(invoice.ID))
		if open.IsPositive() {
			period := repository.memory.periodByID(invoice.BillingPeriodID)
			rows = append(rows, accountv2.OpenInvoice{ID: invoice.ID, ClientID: invoice.ClientID,
				PeriodStart: period.PeriodStart, CreatedAt: invoice.CreatedAt, ProductsTotal: invoice.ProductsTotal, OpenAmount: open})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].PeriodStart.Equal(rows[j].PeriodStart) {
			return rows[i].ID < rows[j].ID
		}
		return rows[i].PeriodStart.Before(rows[j].PeriodStart)
	})
	return rows, nil
}

func (repository *memoryAccountRepository) InvoiceOpenAmount(ctx context.Context, clientID, invoiceID int64) (accountv2.OpenInvoice, error) {
	rows, _ := repository.OpenInvoicesFIFO(ctx, clientID)
	for _, row := range rows {
		if row.ID == invoiceID {
			return row, nil
		}
	}
	return accountv2.OpenInvoice{}, gorm.ErrRecordNotFound
}

func (repository *memoryAccountRepository) AvailablePaymentsFIFO(context.Context, int64) ([]accountv2.PaymentCredit, error) {
	rows := make([]accountv2.PaymentCredit, 0)
	for _, payment := range repository.memory.payments {
		if payment.Status != "posted" {
			continue
		}
		unallocated := payment.Amount.Sub(repository.memory.allocatedFromPayment(payment.ID))
		if unallocated.IsPositive() {
			rows = append(rows, accountv2.PaymentCredit{ID: payment.ID, ClientID: payment.ClientID,
				EffectiveDate: payment.EffectiveDate, CreatedAt: payment.CreatedAt, Amount: payment.Amount, UnallocatedAmount: unallocated})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows, nil
}

func (repository *memoryAccountRepository) PaymentCredit(context.Context, int64, int64) (accountv2.PaymentCredit, error) {
	return accountv2.PaymentCredit{}, gorm.ErrRecordNotFound
}

func (repository *memoryAccountRepository) CreateAllocation(_ context.Context, clientID int64, allocation accountv2.Allocation) error {
	repository.memory.allocations = append(repository.memory.allocations, memoryAllocation{Allocation: allocation, active: true})
	return nil
}

func (repository *memoryAccountRepository) ValidateConservation(context.Context, int64) error {
	return nil
}

func (memory *invoiceMemory) allocatedToInvoice(id int64) decimal.Decimal {
	total := decimal.Zero
	for _, allocation := range memory.allocations {
		if allocation.active && allocation.InvoiceID == id {
			total = total.Add(allocation.Amount)
		}
	}
	return total
}

func (memory *invoiceMemory) allocatedFromPayment(id int64) decimal.Decimal {
	total := decimal.Zero
	for _, allocation := range memory.allocations {
		if allocation.active && allocation.PaymentID == id {
			total = total.Add(allocation.Amount)
		}
	}
	return total
}

func TestIssueSnapshotsNegativeProfitAndConsumesExistingCredit(t *testing.T) {
	memory := newInvoiceMemory()
	memory.payments[50] = domainv2.Payment{ID: 50, ClientID: 1, Amount: decimal.NewFromInt(300), Status: "posted"}
	service := newInvoiceService(t, memory)

	response, err := service.Issue(context.Background(), IssueRequest{ClientID: 1, Year: 2026, Month: 10,
		Products: []IssueProduct{{ProductID: 10, Quantity: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if response.Account.Position != accountv2.PositionDebt || !response.Account.NetBalance.Decimal().Equal(decimal.NewFromInt(200)) {
		t.Fatalf("account = %#v", response.Account)
	}
	if !response.Invoice.PaidAmount.Decimal().Equal(decimal.NewFromInt(300)) || !response.Invoice.OpenAmount.Decimal().Equal(decimal.NewFromInt(200)) {
		t.Fatalf("invoice payment projection = %#v", response.Invoice.InvoiceSummary)
	}
	item := response.Invoice.Items[0]
	if !item.ProfitTotal.Decimal().Equal(decimal.NewFromInt(-100)) || !item.UnitPurchasePrice.Decimal().Equal(decimal.NewFromInt(600)) {
		t.Fatalf("snapshot item = %#v", item)
	}
	memory.products[10] = domainv2.Product{ID: 10, Name: "Changed", PurchasePrice: decimal.Zero, SalePrice: decimal.NewFromInt(1), Active: true}
	detail, err := service.Detail(context.Background(), response.Invoice.ID)
	if err != nil || !detail.Items[0].UnitSalePrice.Decimal().Equal(decimal.NewFromInt(500)) {
		t.Fatalf("historical snapshot changed: detail=%#v err=%v", detail, err)
	}
}

func TestCancelLatestReleasesCreditAndReconciles(t *testing.T) {
	memory := newInvoiceMemory()
	memory.payments[50] = domainv2.Payment{ID: 50, ClientID: 1, Amount: decimal.NewFromInt(600), Status: "posted"}
	service := newInvoiceService(t, memory)
	first, err := service.Issue(context.Background(), IssueRequest{ClientID: 1, Year: 2026, Month: 10,
		Products: []IssueProduct{{ProductID: 10, Quantity: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Issue(context.Background(), IssueRequest{ClientID: 1, Year: 2026, Month: 11,
		Products: []IssueProduct{{ProductID: 20, Quantity: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Cancel(context.Background(), first.Invoice.ID, CancelRequest{Reason: "wrong"}); !errors.Is(err, ErrInvoiceNotLatest) {
		t.Fatalf("non-latest cancellation error = %v", err)
	}
	canceled, err := service.Cancel(context.Background(), second.Invoice.ID, CancelRequest{Reason: " wrong month "})
	if err != nil {
		t.Fatal(err)
	}
	if canceled.Invoice.Status != "canceled" || canceled.Invoice.PaymentStatus != "canceled" {
		t.Fatalf("canceled invoice = %#v", canceled.Invoice.InvoiceSummary)
	}
	if canceled.Account.Position != accountv2.PositionCredit || !canceled.Account.CreditAmount.Decimal().Equal(decimal.NewFromInt(100)) {
		t.Fatalf("account after cancellation = %#v", canceled.Account)
	}
	if !memory.allocatedFromPayment(50).Equal(decimal.NewFromInt(500)) {
		t.Fatalf("active allocations = %s, want 500", memory.allocatedFromPayment(50))
	}
}

func TestIssueRejectsRetroactivePeriodAfterLaterInvoice(t *testing.T) {
	memory := newInvoiceMemory()
	service := newInvoiceService(t, memory)
	_, err := service.Issue(context.Background(), IssueRequest{ClientID: 1, Year: 2026, Month: 11,
		Products: []IssueProduct{{ProductID: 20, Quantity: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Issue(context.Background(), IssueRequest{ClientID: 1, Year: 2026, Month: 10,
		Products: []IssueProduct{{ProductID: 20, Quantity: 1}}})
	if !errors.Is(err, ErrLaterInvoiceExists) {
		t.Fatalf("retroactive issue error = %v", err)
	}
}

func TestIssueRejectsQuantityOutsidePostgresIntegerRange(t *testing.T) {
	memory := newInvoiceMemory()
	service := newInvoiceService(t, memory)

	_, err := service.Issue(context.Background(), IssueRequest{ClientID: 1, Year: 2026, Month: 11,
		Products: []IssueProduct{{ProductID: 20, Quantity: maxQuantity + 1}}})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("quantity outside INTEGER range error = %v", err)
	}
	if len(memory.invoices) != 0 {
		t.Fatalf("invalid quantity persisted %d invoice(s)", len(memory.invoices))
	}
}

func TestConcurrentIssueCreatesOnlyOneActiveInvoicePerMonth(t *testing.T) {
	memory := newInvoiceMemory()
	service := newInvoiceService(t, memory)
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, err := service.Issue(context.Background(), IssueRequest{ClientID: 1, Year: 2026, Month: 11,
				Products: []IssueProduct{{ProductID: 20, Quantity: 1}}})
			results <- err
		}()
	}
	close(start)
	var success, conflict int
	for range 2 {
		switch err := <-results; {
		case err == nil:
			success++
		case errors.Is(err, ErrInvoiceAlreadyExists):
			conflict++
		default:
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d", success, conflict)
	}
}
