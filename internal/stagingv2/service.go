package stagingv2

import (
	"context"
	"errors"
	"strings"
	"time"

	domainv2 "github.com/api-control/internal/domain/v2"
	"github.com/api-control/internal/dto"
	"github.com/shopspring/decimal"
)

var (
	ErrInvalidClient  = errors.New("invalid client")
	ErrInvalidProduct = errors.New("invalid product")
)

type StagingRepository interface {
	UserLookup
	CreateClient(context.Context, *domainv2.Client) error
	FindClient(context.Context, int64) (*domainv2.Client, error)
	ListClients(context.Context) ([]domainv2.Client, error)
	UpdateClient(context.Context, int64, domainv2.Client) error
	SetClientActive(context.Context, int64, bool) error
	CreateProduct(context.Context, *domainv2.Product) error
	FindProduct(context.Context, int64) (*domainv2.Product, error)
	ListProducts(context.Context) ([]domainv2.Product, error)
	UpdateProduct(context.Context, int64, domainv2.Product) error
	SetProductActive(context.Context, int64, bool) error
}

type TokenGenerator func(int64) (string, error)

type Service struct {
	repository    StagingRepository
	authenticator *Authenticator
	tokens        TokenGenerator
}

func NewService(repository StagingRepository, tokens TokenGenerator) (*Service, error) {
	if repository == nil || tokens == nil {
		return nil, errors.New("staging v2 service dependencies are required")
	}
	authenticator, err := NewAuthenticator(repository)
	if err != nil {
		return nil, err
	}
	return &Service{repository: repository, authenticator: authenticator, tokens: tokens}, nil
}

func (service *Service) Login(ctx context.Context, request LoginRequest) (*LoginResponse, error) {
	user, err := service.authenticator.Authenticate(ctx, request.Login, request.Password)
	if err != nil {
		return nil, err
	}
	token, err := service.tokens(user.ID)
	if err != nil {
		return nil, err
	}
	return &LoginResponse{Token: token, User: UserSummary{ID: user.ID, Name: user.Name, Login: user.Login}}, nil
}

func (service *Service) CreateClient(ctx context.Context, request ClientRequest) (*ClientResponse, error) {
	client, err := clientFromRequest(request)
	if err != nil {
		return nil, err
	}
	client.Active = true
	if err := service.repository.CreateClient(ctx, client); err != nil {
		return nil, err
	}
	return clientResponse(*client), nil
}

func (service *Service) FindClient(ctx context.Context, id int64) (*ClientResponse, error) {
	client, err := service.repository.FindClient(ctx, id)
	if err != nil {
		return nil, err
	}
	return clientResponse(*client), nil
}

func (service *Service) ListClients(ctx context.Context) ([]ClientResponse, error) {
	clients, err := service.repository.ListClients(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]ClientResponse, 0, len(clients))
	for _, client := range clients {
		result = append(result, *clientResponse(client))
	}
	return result, nil
}

func (service *Service) UpdateClient(ctx context.Context, id int64, request ClientRequest) error {
	client, err := clientFromRequest(request)
	if err != nil {
		return err
	}
	return service.repository.UpdateClient(ctx, id, *client)
}

func (service *Service) SetClientActive(ctx context.Context, id int64, active bool) error {
	return service.repository.SetClientActive(ctx, id, active)
}

func (service *Service) CreateProduct(ctx context.Context, request ProductRequest) (*ProductResponse, error) {
	product, err := productFromRequest(request)
	if err != nil {
		return nil, err
	}
	product.Active = true
	if err := service.repository.CreateProduct(ctx, product); err != nil {
		return nil, err
	}
	return productResponse(*product), nil
}

func (service *Service) FindProduct(ctx context.Context, id int64) (*ProductResponse, error) {
	product, err := service.repository.FindProduct(ctx, id)
	if err != nil {
		return nil, err
	}
	return productResponse(*product), nil
}

func (service *Service) ListProducts(ctx context.Context) ([]ProductResponse, error) {
	products, err := service.repository.ListProducts(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]ProductResponse, 0, len(products))
	for _, product := range products {
		result = append(result, *productResponse(product))
	}
	return result, nil
}

func (service *Service) UpdateProduct(ctx context.Context, id int64, request ProductRequest) error {
	product, err := productFromRequest(request)
	if err != nil {
		return err
	}
	return service.repository.UpdateProduct(ctx, id, *product)
}

func (service *Service) SetProductActive(ctx context.Context, id int64, active bool) error {
	return service.repository.SetProductActive(ctx, id, active)
}

func clientFromRequest(request ClientRequest) (*domainv2.Client, error) {
	if strings.TrimSpace(request.Name) == "" || normalizeDocument(request.Document) == "" ||
		strings.TrimSpace(request.Phone) == "" || strings.TrimSpace(request.Street) == "" ||
		strings.TrimSpace(request.Neighborhood) == "" || strings.TrimSpace(request.AddressNumber) == "" ||
		strings.TrimSpace(request.AddressType) == "" || request.Position < 0 {
		return nil, ErrInvalidClient
	}
	var birthDate *time.Time
	if request.BirthDate != nil {
		parsed, err := time.Parse("2006-01-02", *request.BirthDate)
		if err != nil {
			return nil, ErrInvalidClient
		}
		birthDate = &parsed
	}
	return &domainv2.Client{
		Name: request.Name, Document: request.Document, BirthDate: birthDate,
		Phone: request.Phone, SecondaryPhone: request.SecondaryPhone, Street: request.Street,
		Neighborhood: request.Neighborhood, AddressNumber: request.AddressNumber,
		Complement: request.Complement, PostalCode: request.PostalCode, AddressType: request.AddressType,
		AddressReference: request.AddressReference, Position: request.Position,
	}, nil
}

func productFromRequest(request ProductRequest) (*domainv2.Product, error) {
	if strings.TrimSpace(request.Name) == "" || request.PurchasePrice == nil || request.SalePrice == nil {
		return nil, ErrInvalidProduct
	}
	purchase := request.PurchasePrice.Decimal()
	sale := request.SalePrice.Decimal()
	if purchase.IsNegative() || !sale.IsPositive() || !fitsUnitPrice(purchase) || !fitsUnitPrice(sale) {
		return nil, ErrInvalidProduct
	}
	return &domainv2.Product{Name: request.Name, PurchasePrice: purchase.Copy(), SalePrice: sale.Copy(), ImageURL: request.ImageURL}, nil
}

func clientResponse(client domainv2.Client) *ClientResponse {
	var birthDate *string
	if client.BirthDate != nil {
		value := client.BirthDate.Format("2006-01-02")
		birthDate = &value
	}
	return &ClientResponse{ID: client.ID, Name: client.Name, Active: client.Active, ClientRequest: ClientRequest{
		Name: client.Name, Document: client.Document, BirthDate: birthDate, Phone: client.Phone,
		SecondaryPhone: client.SecondaryPhone, Street: client.Street, Neighborhood: client.Neighborhood,
		AddressNumber: client.AddressNumber, Complement: client.Complement, PostalCode: client.PostalCode,
		AddressType: client.AddressType, AddressReference: client.AddressReference, Position: client.Position,
	}}
}

func productResponse(product domainv2.Product) *ProductResponse {
	purchase := dto.NewMoney(product.PurchasePrice)
	return &ProductResponse{ID: product.ID, Name: product.Name, PurchasePrice: &purchase,
		SalePrice: dto.NewMoney(product.SalePrice), ImageURL: product.ImageURL, Active: product.Active}
}

func fitsUnitPrice(value decimal.Decimal) bool {
	return value.Abs().LessThanOrEqual(decimal.RequireFromString("999999999999.99")) && value.Exponent() >= -2
}
