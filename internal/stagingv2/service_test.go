package stagingv2

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	domainv2 "github.com/api-control/internal/domain/v2"
	"github.com/api-control/internal/dto"
	"github.com/shopspring/decimal"
)

type stagingRepositoryFake struct {
	user           *domainv2.User
	createdProduct *domainv2.Product
	createdClient  *domainv2.Client
}

func (fake *stagingRepositoryFake) FindActiveUserByLogin(context.Context, string) (*domainv2.User, error) {
	return fake.user, nil
}
func (fake *stagingRepositoryFake) CreateClient(_ context.Context, client *domainv2.Client) error {
	client.ID = 1
	fake.createdClient = client
	return nil
}
func (fake *stagingRepositoryFake) FindClient(context.Context, int64) (*domainv2.Client, error) {
	return fake.createdClient, nil
}
func (fake *stagingRepositoryFake) ListClients(context.Context) ([]domainv2.Client, error) {
	return nil, nil
}
func (fake *stagingRepositoryFake) UpdateClient(context.Context, int64, domainv2.Client) error {
	return nil
}
func (fake *stagingRepositoryFake) SetClientActive(context.Context, int64, bool) error { return nil }
func (fake *stagingRepositoryFake) CreateProduct(_ context.Context, product *domainv2.Product) error {
	product.ID = 2
	fake.createdProduct = product
	return nil
}
func (fake *stagingRepositoryFake) FindProduct(context.Context, int64) (*domainv2.Product, error) {
	return fake.createdProduct, nil
}
func (fake *stagingRepositoryFake) ListProducts(context.Context) ([]domainv2.Product, error) {
	return nil, nil
}
func (fake *stagingRepositoryFake) UpdateProduct(context.Context, int64, domainv2.Product) error {
	return nil
}
func (fake *stagingRepositoryFake) SetProductActive(context.Context, int64, bool) error { return nil }

func TestStagingProductServiceKeepsMoneyNumeric(t *testing.T) {
	fake := &stagingRepositoryFake{}
	service, err := NewService(fake, func(int64) (string, error) { return "token", nil })
	if err != nil {
		t.Fatal(err)
	}
	purchase := dto.NewMoney(decimal.RequireFromString("7.25"))
	sale := dto.NewMoney(decimal.RequireFromString("12.50"))
	response, err := service.CreateProduct(context.Background(), ProductRequest{Name: "Product", PurchasePrice: &purchase, SalePrice: &sale})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"purchasePrice":"`) || strings.Contains(string(encoded), `"salePrice":"`) {
		t.Fatalf("money encoded as string: %s", encoded)
	}
	if fake.createdProduct == nil || !fake.createdProduct.PurchasePrice.Equal(decimal.RequireFromString("7.25")) {
		t.Fatalf("created product = %+v", fake.createdProduct)
	}
}

func TestStagingClientServiceParsesCivilBirthDate(t *testing.T) {
	fake := &stagingRepositoryFake{}
	service, err := NewService(fake, func(int64) (string, error) { return "token", nil })
	if err != nil {
		t.Fatal(err)
	}
	birthDate := "2000-02-29"
	response, err := service.CreateClient(context.Background(), ClientRequest{
		Name: "Client", Document: "123.456.789-00", BirthDate: &birthDate, Phone: "1",
		Street: "Street", Neighborhood: "Center", AddressNumber: "10", AddressType: "home",
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.BirthDate == nil || *response.BirthDate != birthDate {
		t.Fatalf("birth date = %v", response.BirthDate)
	}
}
