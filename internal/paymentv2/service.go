package paymentv2

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/api-control/internal/accountv2"
	domainv2 "github.com/api-control/internal/domain/v2"
)

type Clock func() time.Time

type Service struct {
	unitOfWork UnitOfWork
	queries    QueryRepository
	allocation *accountv2.Engine
	now        Clock
	location   *time.Location
}

func NewService(unitOfWork UnitOfWork, queries QueryRepository, now Clock) (*Service, error) {
	if unitOfWork == nil || queries == nil {
		return nil, errors.New("payment v2 dependencies are required")
	}
	if now == nil {
		now = time.Now
	}
	location, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return nil, err
	}
	return &Service{unitOfWork: unitOfWork, queries: queries, allocation: accountv2.NewEngine(), now: now, location: location}, nil
}

func (service *Service) Create(ctx context.Context, request CreateRequest) (*MutationResponse, error) {
	effectiveDate, err := service.validateCreate(request)
	if err != nil {
		return nil, err
	}
	var response MutationResponse
	err = service.unitOfWork.WithLockedClient(ctx, request.ClientID, func(repository Repository, accountRepository accountv2.AccountRepository) error {
		client, err := repository.FindClient(ctx, request.ClientID)
		if err != nil {
			return err
		}
		payment := &domainv2.Payment{ClientID: request.ClientID, Amount: request.Amount.Decimal().Copy(),
			EffectiveDate: effectiveDate, Observation: request.Observation, Status: "posted"}
		if err := repository.CreatePayment(ctx, payment); err != nil {
			return err
		}
		if _, err := service.allocation.AllocatePaymentIn(ctx, accountRepository, request.ClientID, payment.ID); err != nil {
			return err
		}
		position, err := accountRepository.Position(ctx, request.ClientID)
		if err != nil {
			return err
		}
		record, err := repository.FindPayment(ctx, payment.ID)
		if err != nil {
			return err
		}
		allocations, err := repository.FindActiveAllocations(ctx, payment.ID)
		if err != nil {
			return err
		}
		paymentResponse := responseFromRecord(*record, allocations)
		paymentResponse.Client = ClientSummary{ID: client.ID, Name: client.Name, Active: client.Active}
		response = MutationResponse{PaymentResponse: paymentResponse, Account: accountResponse(position)}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &response, nil
}

func (service *Service) Reverse(ctx context.Context, paymentID int64, request ReverseRequest) (*MutationResponse, error) {
	reason := strings.TrimSpace(request.Reason)
	if paymentID <= 0 || reason == "" {
		return nil, ErrInvalidRequest
	}
	clientID, err := service.queries.FindPaymentClientID(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	var response MutationResponse
	err = service.unitOfWork.WithLockedClient(ctx, clientID, func(repository Repository, accountRepository accountv2.AccountRepository) error {
		record, err := repository.FindPayment(ctx, paymentID)
		if err != nil {
			return err
		}
		if record.ClientID != clientID {
			return accountv2.ErrInconsistentAccountSession
		}
		if record.Status != "posted" {
			return ErrPaymentNotPosted
		}
		at := service.now().UTC()
		if err := repository.ReversePayment(ctx, paymentID, at, reason); err != nil {
			return err
		}
		if err := repository.ReverseActiveAllocations(ctx, paymentID, at, "payment reversed"); err != nil {
			return err
		}
		if err := repository.ReverseOtherActiveAllocations(ctx, clientID, paymentID, at, "FIFO rebuilt after payment reversal"); err != nil {
			return err
		}
		if _, err := service.allocation.ReconcileIn(ctx, accountRepository, clientID); err != nil {
			return err
		}
		position, err := accountRepository.Position(ctx, clientID)
		if err != nil {
			return err
		}
		reversed, err := repository.FindPayment(ctx, paymentID)
		if err != nil {
			return err
		}
		response = MutationResponse{PaymentResponse: responseFromRecord(*reversed, nil), Account: accountResponse(position)}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &response, nil
}

func (service *Service) Detail(ctx context.Context, paymentID int64) (*PaymentResponse, error) {
	if paymentID <= 0 {
		return nil, ErrInvalidRequest
	}
	record, err := service.queries.FindPayment(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	allocations, err := service.queries.FindActiveAllocations(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	response := responseFromRecord(*record, allocations)
	return &response, nil
}

func (service *Service) List(ctx context.Context, filter ListFilter) (*ListResponse, error) {
	if filter.Limit < 0 || filter.Limit > maxLimit || (filter.ClientID != nil && *filter.ClientID <= 0) ||
		(filter.Status != "" && filter.Status != "posted" && filter.Status != "reversed") {
		return nil, ErrInvalidFilter
	}
	if filter.DateFrom != nil && filter.DateTo != nil {
		if filter.DateFrom.After(*filter.DateTo) || filter.DateTo.Sub(*filter.DateFrom) > 366*24*time.Hour {
			return nil, ErrInvalidFilter
		}
	}
	cursor, err := decodeCursor(filter.Cursor)
	if err != nil {
		return nil, err
	}
	limit := filter.Limit
	if limit == 0 {
		limit = defaultLimit
	}
	records, err := service.queries.ListPayments(ctx, listQuery{ClientID: filter.ClientID, DateFrom: filter.DateFrom,
		DateTo: filter.DateTo, Status: filter.Status, Cursor: cursor, Limit: limit + 1})
	if err != nil {
		return nil, err
	}
	response := &ListResponse{Items: make([]PaymentResponse, 0, min(len(records), limit))}
	visible := records
	if len(records) > limit {
		visible = records[:limit]
		next := encodeCursor(visible[len(visible)-1])
		response.NextCursor = &next
	}
	for _, record := range visible {
		response.Items = append(response.Items, responseFromRecord(record, nil))
	}
	return response, nil
}

func (service *Service) validateCreate(request CreateRequest) (time.Time, error) {
	if request.ClientID <= 0 || request.Amount == nil || !request.Amount.Decimal().IsPositive() {
		return time.Time{}, ErrInvalidRequest
	}
	effectiveDate, err := time.Parse("2006-01-02", request.EffectiveDate)
	if err != nil {
		return time.Time{}, ErrInvalidRequest
	}
	today := service.now().In(service.location)
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	if effectiveDate.After(todayDate) {
		return time.Time{}, ErrFutureEffectiveDate
	}
	return effectiveDate, nil
}

func responseFromRecord(record paymentRecord, allocations []accountv2.Allocation) PaymentResponse {
	allocationResponses := make([]AllocationResponse, 0, len(allocations))
	for _, allocation := range allocations {
		allocationResponses = append(allocationResponses, AllocationResponse{InvoiceID: allocation.InvoiceID,
			Amount: accountv2.NewJSONAmount(allocation.Amount)})
	}
	credit := record.Amount.Sub(record.AllocatedAmount)
	if record.Status == "reversed" {
		credit = record.Amount.Sub(record.Amount)
	}
	return PaymentResponse{ID: record.ID, Client: ClientSummary{ID: record.ClientID, Name: record.ClientName, Active: record.ClientActive},
		Amount: accountv2.NewJSONAmount(record.Amount), EffectiveDate: record.EffectiveDate.Format("2006-01-02"),
		Observation: record.Observation, Status: record.Status, AllocatedAmount: accountv2.NewJSONAmount(record.AllocatedAmount),
		CreditAmount: accountv2.NewJSONAmount(credit), Allocations: allocationResponses, CreatedAt: record.CreatedAt,
		ReversedAt: record.ReversedAt, ReversalReason: record.ReversalReason}
}

func accountResponse(position accountv2.Position) AccountResponse {
	return AccountResponse{Position: position.State, NetBalance: accountv2.NewJSONAmount(position.NetBalance),
		DebtAmount: accountv2.NewJSONAmount(position.DebtAmount), CreditAmount: accountv2.NewJSONAmount(position.CreditAmount)}
}
