package invoicev2

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/api-control/internal/accountv2"
	domainv2 "github.com/api-control/internal/domain/v2"
	"github.com/shopspring/decimal"
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
		return nil, errors.New("invoice v2 dependencies are required")
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

func (service *Service) Issue(ctx context.Context, request IssueRequest) (*MutationResponse, error) {
	period, productQuantities, err := service.validateIssue(request)
	if err != nil {
		return nil, err
	}
	var response MutationResponse
	err = service.unitOfWork.WithLockedClient(ctx, request.ClientID, func(repository Repository, accountRepository accountv2.AccountRepository) error {
		client, err := repository.FindClient(ctx, request.ClientID)
		if err != nil {
			return err
		}
		if !client.Active {
			return ErrInactiveClient
		}
		billingPeriod, err := repository.FindOrCreatePeriod(ctx, period)
		if err != nil {
			return err
		}
		exists, err := repository.HasActiveInvoiceInPeriod(ctx, request.ClientID, billingPeriod.ID)
		if err != nil {
			return err
		}
		if exists {
			return ErrInvoiceAlreadyExists
		}
		later, err := repository.HasLaterActiveInvoice(ctx, request.ClientID, period)
		if err != nil {
			return err
		}
		if later {
			return ErrLaterInvoiceExists
		}

		productIDs := make([]int64, 0, len(productQuantities))
		for id := range productQuantities {
			productIDs = append(productIDs, id)
		}
		sort.Slice(productIDs, func(i, j int) bool { return productIDs[i] < productIDs[j] })
		products, err := repository.FindActiveProducts(ctx, productIDs)
		if err != nil {
			return err
		}
		if len(products) != len(productIDs) {
			return ErrInactiveProduct
		}

		items, productsTotal, err := snapshotItems(products, productQuantities)
		if err != nil {
			return err
		}
		positionBefore, err := accountRepository.Position(ctx, request.ClientID)
		if err != nil {
			return err
		}
		invoice := &domainv2.Invoice{
			ClientID: request.ClientID, BillingPeriodID: billingPeriod.ID, Status: "issued",
			Observation: request.Observation, AccountBalanceBeforeIssue: positionBefore.NetBalance.Copy(),
			ProductsTotal: productsTotal.Copy(), AccountBalanceAfterCharge: positionBefore.NetBalance.Add(productsTotal),
		}
		if err := repository.CreateInvoice(ctx, invoice); err != nil {
			return err
		}
		for index := range items {
			items[index].InvoiceID = invoice.ID
		}
		if err := repository.CreateItems(ctx, items); err != nil {
			return err
		}
		persistedTotal, err := repository.PersistedSaleTotal(ctx, invoice.ID)
		if err != nil {
			return err
		}
		if !persistedTotal.Equal(productsTotal) {
			return ErrPersistedTotal
		}
		if _, err := service.allocation.AllocateInvoiceIn(ctx, accountRepository, request.ClientID, invoice.ID); err != nil {
			return err
		}
		position, err := accountRepository.Position(ctx, request.ClientID)
		if err != nil {
			return err
		}
		detail, err := detailFromRepository(ctx, repository, invoice.ID)
		if err != nil {
			return err
		}
		response = MutationResponse{Invoice: *detail, Account: accountResponse(position)}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &response, nil
}

func (service *Service) Cancel(ctx context.Context, invoiceID int64, request CancelRequest) (*MutationResponse, error) {
	reason := strings.TrimSpace(request.Reason)
	if invoiceID <= 0 || reason == "" {
		return nil, ErrInvalidRequest
	}
	clientID, err := service.queries.FindInvoiceClientID(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	var response MutationResponse
	err = service.unitOfWork.WithLockedClient(ctx, clientID, func(repository Repository, accountRepository accountv2.AccountRepository) error {
		record, err := repository.FindInvoice(ctx, invoiceID)
		if err != nil {
			return err
		}
		if record.ClientID != clientID {
			return accountv2.ErrInconsistentAccountSession
		}
		if record.Status != "issued" {
			return ErrInvoiceNotIssued
		}
		latestID, err := repository.LatestActiveInvoiceID(ctx, clientID)
		if err != nil {
			return err
		}
		if latestID != invoiceID {
			return ErrInvoiceNotLatest
		}
		at := service.now().UTC()
		if err := repository.CancelInvoice(ctx, invoiceID, at, reason); err != nil {
			return err
		}
		if err := repository.ReverseActiveAllocations(ctx, invoiceID, at, "invoice canceled"); err != nil {
			return err
		}
		if _, err := service.allocation.ReconcileIn(ctx, accountRepository, clientID); err != nil {
			return err
		}
		position, err := accountRepository.Position(ctx, clientID)
		if err != nil {
			return err
		}
		detail, err := detailFromRepository(ctx, repository, invoiceID)
		if err != nil {
			return err
		}
		response = MutationResponse{Invoice: *detail, Account: accountResponse(position)}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &response, nil
}

func (service *Service) Detail(ctx context.Context, invoiceID int64) (*InvoiceDetail, error) {
	if invoiceID <= 0 {
		return nil, ErrInvalidRequest
	}
	record, err := service.queries.FindInvoice(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	items, err := service.queries.FindItems(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	return detailResponse(*record, items), nil
}

func (service *Service) List(ctx context.Context, filter ListFilter) (*ListResponse, error) {
	period, err := validPeriod(filter.Year, filter.Month)
	if err != nil || filter.Limit < 0 || filter.Limit > maxLimit || (filter.ClientID != nil && *filter.ClientID <= 0) {
		return nil, ErrInvalidRequest
	}
	cursor, err := decodeCursor(filter.Cursor)
	if err != nil {
		return nil, err
	}
	limit := filter.Limit
	if limit == 0 {
		limit = defaultLimit
	}
	records, err := service.queries.ListInvoices(ctx, listQuery{PeriodStart: period, ClientID: filter.ClientID, Cursor: cursor, Limit: limit + 1})
	if err != nil {
		return nil, err
	}
	response := &ListResponse{Items: make([]InvoiceSummary, 0, min(len(records), limit))}
	visible := records
	if len(records) > limit {
		visible = records[:limit]
		last := visible[len(visible)-1]
		next := encodeCursor(last.CreatedAt, last.ID)
		response.NextCursor = &next
	}
	for _, record := range visible {
		response.Items = append(response.Items, summaryResponse(record))
	}
	return response, nil
}

func (service *Service) validateIssue(request IssueRequest) (time.Time, map[int64]int, error) {
	period, err := validPeriod(request.Year, request.Month)
	if err != nil || request.ClientID <= 0 || len(request.Products) == 0 {
		return time.Time{}, nil, ErrInvalidRequest
	}
	today := service.now().In(service.location)
	currentPeriod := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	if period.After(currentPeriod) {
		return time.Time{}, nil, ErrFuturePeriod
	}
	quantities := make(map[int64]int, len(request.Products))
	for _, product := range request.Products {
		if product.ProductID <= 0 || product.Quantity <= 0 || product.Quantity > maxQuantity {
			return time.Time{}, nil, ErrInvalidRequest
		}
		if _, duplicate := quantities[product.ProductID]; duplicate {
			return time.Time{}, nil, ErrInvalidRequest
		}
		quantities[product.ProductID] = product.Quantity
	}
	return period, quantities, nil
}

func validPeriod(year, month int) (time.Time, error) {
	if year < 1 || year > 9999 || month < 1 || month > 12 {
		return time.Time{}, ErrInvalidRequest
	}
	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC), nil
}

func snapshotItems(products []domainv2.Product, quantities map[int64]int) ([]domainv2.InvoiceItem, decimal.Decimal, error) {
	items := make([]domainv2.InvoiceItem, 0, len(products))
	total := decimal.Zero
	maxTotal := decimal.RequireFromString("9999999999999.99")
	for _, product := range products {
		quantity, exists := quantities[product.ID]
		if !exists || quantity <= 0 || product.PurchasePrice.IsNegative() || !product.SalePrice.IsPositive() {
			return nil, decimal.Zero, ErrInactiveProduct
		}
		purchaseTotal := product.PurchasePrice.Mul(decimal.NewFromInt(int64(quantity)))
		saleTotal := product.SalePrice.Mul(decimal.NewFromInt(int64(quantity)))
		if purchaseTotal.GreaterThan(maxTotal) || saleTotal.GreaterThan(maxTotal) {
			return nil, decimal.Zero, ErrInvalidRequest
		}
		total = total.Add(saleTotal)
		if total.GreaterThan(maxTotal) {
			return nil, decimal.Zero, ErrInvalidRequest
		}
		items = append(items, domainv2.InvoiceItem{ProductID: product.ID, ProductName: product.Name,
			Quantity: quantity, UnitPurchasePrice: product.PurchasePrice.Copy(), UnitSalePrice: product.SalePrice.Copy()})
	}
	return items, total, nil
}

func detailFromRepository(ctx context.Context, repository Repository, invoiceID int64) (*InvoiceDetail, error) {
	record, err := repository.FindInvoice(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	items, err := repository.FindItems(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	return detailResponse(*record, items), nil
}

func detailResponse(record invoiceRecord, items []domainv2.InvoiceItem) *InvoiceDetail {
	responseItems := make([]InvoiceItemResponse, 0, len(items))
	for _, item := range items {
		responseItems = append(responseItems, InvoiceItemResponse{ID: item.ID, ProductID: item.ProductID,
			ProductName: item.ProductName, Quantity: item.Quantity, UnitPurchasePrice: newAmount(item.UnitPurchasePrice),
			UnitSalePrice: newAmount(item.UnitSalePrice), PurchaseTotal: newAmount(item.PurchaseTotal),
			SaleTotal: newAmount(item.SaleTotal), ProfitTotal: newAmount(item.ProfitTotal)})
	}
	return &InvoiceDetail{InvoiceSummary: summaryResponse(record), Items: responseItems}
}

func summaryResponse(record invoiceRecord) InvoiceSummary {
	return InvoiceSummary{ID: record.ID, Status: record.Status,
		Period:        PeriodResponse{Year: record.PeriodStart.Year(), Month: int(record.PeriodStart.Month())},
		Client:        ClientSummary{ID: record.ClientID, Name: record.ClientName, Active: record.ClientActive},
		ProductsTotal: newAmount(record.ProductsTotal), AccountBalanceBeforeIssue: newAmount(record.AccountBalanceBeforeIssue),
		AccountBalanceAfterCharge: newAmount(record.AccountBalanceAfterCharge), PaidAmount: newAmount(record.PaidAmount),
		OpenAmount: newAmount(record.OpenAmount), PaymentStatus: paymentStatus(record), Observation: record.Observation,
		CreatedAt: record.CreatedAt, CanceledAt: record.CanceledAt, CancellationReason: record.CancellationReason}
}

func paymentStatus(record invoiceRecord) string {
	if record.Status == "canceled" {
		return "canceled"
	}
	if record.PaidAmount.IsZero() {
		return "open"
	}
	if record.OpenAmount.IsZero() {
		return "paid"
	}
	return "partially_paid"
}
