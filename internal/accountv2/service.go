package accountv2

import (
	"context"
	"errors"

	"github.com/shopspring/decimal"
)

type Result struct {
	Allocations []Allocation
	Position    Position
}

type Engine struct{}

func NewEngine() *Engine { return &Engine{} }

// AllocatePaymentIn is reusable by the payment service while it is already
// inside the same locked client transaction that creates the payment.
func (engine *Engine) AllocatePaymentIn(ctx context.Context, repository AccountRepository, clientID, paymentID int64) ([]Allocation, error) {
	if clientID <= 0 {
		return nil, ErrInvalidClientID
	}
	if paymentID <= 0 {
		return nil, ErrInvalidPaymentID
	}
	payment, err := repository.PaymentCredit(ctx, clientID, paymentID)
	if err != nil {
		return nil, err
	}
	if payment.ClientID != clientID {
		return nil, ErrInconsistentAccountSession
	}
	invoices, err := repository.OpenInvoicesFIFO(ctx, clientID)
	if err != nil {
		return nil, err
	}
	allocations, err := allocatePaymentCredit(ctx, repository, clientID, payment, invoices)
	if err != nil {
		return nil, err
	}
	if err := repository.ValidateConservation(ctx, clientID); err != nil {
		return nil, err
	}
	return allocations, nil
}

// AllocateInvoiceIn applies existing payment credit in payment FIFO order.
// It is intended for the invoice issue transaction after the invoice exists.
func (engine *Engine) AllocateInvoiceIn(ctx context.Context, repository AccountRepository, clientID, invoiceID int64) ([]Allocation, error) {
	if clientID <= 0 {
		return nil, ErrInvalidClientID
	}
	if invoiceID <= 0 {
		return nil, ErrInvalidInvoiceID
	}
	invoice, err := repository.InvoiceOpenAmount(ctx, clientID, invoiceID)
	if err != nil {
		return nil, err
	}
	if invoice.ClientID != clientID {
		return nil, ErrInconsistentAccountSession
	}
	payments, err := repository.AvailablePaymentsFIFO(ctx, clientID)
	if err != nil {
		return nil, err
	}
	allocations := make([]Allocation, 0)
	remaining := invoice.OpenAmount.Copy()
	for _, payment := range payments {
		if !remaining.IsPositive() {
			break
		}
		if payment.ClientID != clientID {
			return nil, ErrInconsistentAccountSession
		}
		amount := decimal.Min(remaining, payment.UnallocatedAmount)
		if !amount.IsPositive() {
			continue
		}
		allocation := Allocation{PaymentID: payment.ID, InvoiceID: invoice.ID, Amount: amount.Copy()}
		if err := repository.CreateAllocation(ctx, clientID, allocation); err != nil {
			return nil, err
		}
		allocations = append(allocations, allocation)
		remaining = remaining.Sub(amount)
	}
	if err := repository.ValidateConservation(ctx, clientID); err != nil {
		return nil, err
	}
	return allocations, nil
}

// ReconcileIn consumes every available payment against open invoices. It is
// used after reversals/cancellations release or invalidate allocations.
func (engine *Engine) ReconcileIn(ctx context.Context, repository AccountRepository, clientID int64) ([]Allocation, error) {
	if clientID <= 0 {
		return nil, ErrInvalidClientID
	}
	pays, err := repository.AvailablePaymentsFIFO(ctx, clientID)
	if err != nil {
		return nil, err
	}
	invoices, err := repository.OpenInvoicesFIFO(ctx, clientID)
	if err != nil {
		return nil, err
	}
	allocations := make([]Allocation, 0)
	invoiceIndex := 0
	for _, payment := range pays {
		if payment.ClientID != clientID {
			return nil, ErrInconsistentAccountSession
		}
		remainingCredit := payment.UnallocatedAmount.Copy()
		for remainingCredit.IsPositive() && invoiceIndex < len(invoices) {
			invoice := &invoices[invoiceIndex]
			if invoice.ClientID != clientID {
				return nil, ErrInconsistentAccountSession
			}
			if !invoice.OpenAmount.IsPositive() {
				invoiceIndex++
				continue
			}
			amount := decimal.Min(remainingCredit, invoice.OpenAmount)
			allocation := Allocation{PaymentID: payment.ID, InvoiceID: invoice.ID, Amount: amount.Copy()}
			if err := repository.CreateAllocation(ctx, clientID, allocation); err != nil {
				return nil, err
			}
			allocations = append(allocations, allocation)
			remainingCredit = remainingCredit.Sub(amount)
			invoice.OpenAmount = invoice.OpenAmount.Sub(amount)
			if !invoice.OpenAmount.IsPositive() {
				invoiceIndex++
			}
		}
	}
	if err := repository.ValidateConservation(ctx, clientID); err != nil {
		return nil, err
	}
	return allocations, nil
}

func allocatePaymentCredit(ctx context.Context, repository AccountRepository, clientID int64, payment PaymentCredit, invoices []OpenInvoice) ([]Allocation, error) {
	remaining := payment.UnallocatedAmount.Copy()
	allocations := make([]Allocation, 0)
	for _, invoice := range invoices {
		if !remaining.IsPositive() {
			break
		}
		if invoice.ClientID != clientID {
			return nil, ErrInconsistentAccountSession
		}
		amount := decimal.Min(remaining, invoice.OpenAmount)
		if !amount.IsPositive() {
			continue
		}
		allocation := Allocation{PaymentID: payment.ID, InvoiceID: invoice.ID, Amount: amount.Copy()}
		if err := repository.CreateAllocation(ctx, clientID, allocation); err != nil {
			return nil, err
		}
		allocations = append(allocations, allocation)
		remaining = remaining.Sub(amount)
	}
	return allocations, nil
}

type Service struct {
	unitOfWork ClientUnitOfWork
	engine     *Engine
}

func NewService(unitOfWork ClientUnitOfWork) (*Service, error) {
	if unitOfWork == nil {
		return nil, errors.New("account v2 unit of work is required")
	}
	return &Service{unitOfWork: unitOfWork, engine: NewEngine()}, nil
}

func (service *Service) Position(ctx context.Context, clientID int64) (Position, error) {
	var position Position
	err := service.unitOfWork.WithLockedClient(ctx, clientID, func(repository AccountRepository) error {
		var err error
		position, err = repository.Position(ctx, clientID)
		return err
	})
	return position, err
}

func (service *Service) AllocatePayment(ctx context.Context, clientID, paymentID int64) (Result, error) {
	return service.run(ctx, clientID, func(repository AccountRepository) ([]Allocation, error) {
		return service.engine.AllocatePaymentIn(ctx, repository, clientID, paymentID)
	})
}

func (service *Service) AllocateInvoice(ctx context.Context, clientID, invoiceID int64) (Result, error) {
	return service.run(ctx, clientID, func(repository AccountRepository) ([]Allocation, error) {
		return service.engine.AllocateInvoiceIn(ctx, repository, clientID, invoiceID)
	})
}

func (service *Service) Reconcile(ctx context.Context, clientID int64) (Result, error) {
	return service.run(ctx, clientID, func(repository AccountRepository) ([]Allocation, error) {
		return service.engine.ReconcileIn(ctx, repository, clientID)
	})
}

func (service *Service) run(ctx context.Context, clientID int64, operation func(AccountRepository) ([]Allocation, error)) (Result, error) {
	result := Result{Allocations: make([]Allocation, 0)}
	err := service.unitOfWork.WithLockedClient(ctx, clientID, func(repository AccountRepository) error {
		allocations, err := operation(repository)
		if err != nil {
			return err
		}
		result.Allocations = allocations
		result.Position, err = repository.Position(ctx, clientID)
		return err
	})
	return result, err
}
