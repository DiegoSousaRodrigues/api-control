package service

import (
	"errors"
	"testing"

	"github.com/api-control/internal/domain"
	"github.com/api-control/internal/dto"
	"github.com/api-control/internal/repository"
	"github.com/shopspring/decimal"
)

var (
	errListRepository   = errors.New("repository list failed")
	errUpdateRepository = errors.New("repository update failed")
)

type failingClientRepository struct{}

func (f *failingClientRepository) List() (*[]domain.Client, error)         { return nil, errListRepository }
func (f *failingClientRepository) Add(domain.Client) error                 { return nil }
func (f *failingClientRepository) FindByID(string) (*domain.Client, error) { return nil, nil }
func (f *failingClientRepository) ChangeStatus(int64, bool) error          { return nil }
func (f *failingClientRepository) Update(int64, domain.Client) error       { return errUpdateRepository }

type failingSkuRepository struct{}

func (f *failingSkuRepository) List() (*[]domain.Sku, error)         { return nil, errListRepository }
func (f *failingSkuRepository) Add(domain.Sku) error                 { return nil }
func (f *failingSkuRepository) ChangeStatus(int64, bool) error       { return nil }
func (f *failingSkuRepository) FindByID(string) (*domain.Sku, error) { return nil, nil }
func (f *failingSkuRepository) Update(int64, domain.Sku) error       { return errUpdateRepository }

type skuLookupErrorRepository struct{}

func (f *skuLookupErrorRepository) List() (*[]domain.Sku, error)   { return nil, nil }
func (f *skuLookupErrorRepository) Add(domain.Sku) error           { return nil }
func (f *skuLookupErrorRepository) ChangeStatus(int64, bool) error { return nil }
func (f *skuLookupErrorRepository) FindByID(string) (*domain.Sku, error) {
	return nil, errUpdateRepository
}
func (f *skuLookupErrorRepository) Update(int64, domain.Sku) error { return nil }

type failingOrderRepository struct {
	updateCalls int
	received    *domain.Order
}

func (f *failingOrderRepository) List(int16, int16) (*[]domain.Order, error) {
	return nil, errListRepository
}
func (f *failingOrderRepository) Add(domain.Order) error { return nil }
func (f *failingOrderRepository) OpenBalance(int64, int16, int16) (decimal.Decimal, error) {
	return decimal.Zero, nil
}
func (f *failingOrderRepository) FindByID(string) (*domain.Order, error) { return nil, nil }
func (f *failingOrderRepository) Update(int64, domain.Order) error {
	f.updateCalls++
	return errUpdateRepository
}

type capturingOrderRepository struct {
	received *domain.Order
}

func (f *capturingOrderRepository) List(int16, int16) (*[]domain.Order, error) { return nil, nil }
func (f *capturingOrderRepository) Add(entity domain.Order) error              { f.received = &entity; return nil }
func (f *capturingOrderRepository) OpenBalance(int64, int16, int16) (decimal.Decimal, error) {
	return decimal.Zero, nil
}
func (f *capturingOrderRepository) FindByID(string) (*domain.Order, error) { return nil, nil }
func (f *capturingOrderRepository) Update(_ int64, entity domain.Order) error {
	f.received = &entity
	return nil
}

func TestClientUpdatePropagatesRepositoryError(t *testing.T) {
	originalRepository := repository.ClientRepository
	repository.ClientRepository = &failingClientRepository{}
	t.Cleanup(func() { repository.ClientRepository = originalRepository })

	err := (&clientService{}).Update("2", dto.ClientDTO{})
	if !errors.Is(err, errUpdateRepository) {
		t.Fatalf("Update error = %v, want %v", err, errUpdateRepository)
	}
}

func TestClientListPropagatesRepositoryError(t *testing.T) {
	originalRepository := repository.ClientRepository
	repository.ClientRepository = &failingClientRepository{}
	t.Cleanup(func() { repository.ClientRepository = originalRepository })

	result, err := (&clientService{}).List()
	if result != nil {
		t.Fatalf("List result = %v, want nil", result)
	}
	if !errors.Is(err, errListRepository) {
		t.Fatalf("List error = %v, want %v", err, errListRepository)
	}
}

func TestSkuUpdatePropagatesRepositoryError(t *testing.T) {
	originalRepository := repository.SkuRepository
	repository.SkuRepository = &failingSkuRepository{}
	t.Cleanup(func() { repository.SkuRepository = originalRepository })

	purchase := dto.NewMoney(decimal.NewFromInt(5))
	sale := dto.NewMoney(decimal.NewFromInt(10))
	err := (&skuService{}).Update("2", dto.SkuUpload{Product: dto.SkuProductRequest{Name: "Product", PurchasePrice: &purchase, SalePrice: &sale}})
	if !errors.Is(err, errUpdateRepository) {
		t.Fatalf("Update error = %v, want %v", err, errUpdateRepository)
	}
}

func TestSkuListPropagatesRepositoryError(t *testing.T) {
	originalRepository := repository.SkuRepository
	repository.SkuRepository = &failingSkuRepository{}
	t.Cleanup(func() { repository.SkuRepository = originalRepository })

	result, err := (&skuService{}).List()
	if result != nil {
		t.Fatalf("List result = %v, want nil", result)
	}
	if !errors.Is(err, errListRepository) {
		t.Fatalf("List error = %v, want %v", err, errListRepository)
	}
}

func TestOrderUpdatePropagatesRepositoryError(t *testing.T) {
	fakeRepository := &failingOrderRepository{}
	originalRepository := repository.OrderRepository
	repository.OrderRepository = fakeRepository
	t.Cleanup(func() { repository.OrderRepository = originalRepository })

	err := (&orderService{}).Update("2", dto.OrderRequestDTO{})
	if !errors.Is(err, repository.ErrOrderFinancialUpdateUnsupported) {
		t.Fatalf("Update error = %v, want restricted update", err)
	}
	if fakeRepository.updateCalls != 0 {
		t.Fatalf("repository Update calls = %d, want 0", fakeRepository.updateCalls)
	}
}

func TestOrderListPropagatesRepositoryError(t *testing.T) {
	originalRepository := repository.OrderRepository
	repository.OrderRepository = &failingOrderRepository{}
	t.Cleanup(func() { repository.OrderRepository = originalRepository })

	result, err := (&orderService{}).List(2020, 1)
	if result != nil {
		t.Fatalf("List result = %v, want nil", result)
	}
	if !errors.Is(err, errListRepository) {
		t.Fatalf("List error = %v, want %v", err, errListRepository)
	}
}

func TestOrderUpdatePropagatesParseError(t *testing.T) {
	fakeRepository := &failingOrderRepository{}
	originalRepository := repository.OrderRepository
	repository.OrderRepository = fakeRepository
	t.Cleanup(func() { repository.OrderRepository = originalRepository })

	err := (&orderService{}).Update("invalid", dto.OrderRequestDTO{})
	if err == nil {
		t.Fatal("Update error = nil, want parse error")
	}
	if fakeRepository.updateCalls != 0 {
		t.Fatalf("repository Update calls = %d, want 0", fakeRepository.updateCalls)
	}
}

func TestOrderUpdateLeavesSkuSnapshotToRepository(t *testing.T) {
	fakeOrderRepository := &capturingOrderRepository{}
	originalOrderRepository := repository.OrderRepository
	repository.OrderRepository = fakeOrderRepository
	t.Cleanup(func() { repository.OrderRepository = originalOrderRepository })

	err := (&orderService{}).Update("2", dto.OrderRequestDTO{})
	if !errors.Is(err, repository.ErrOrderFinancialUpdateUnsupported) {
		t.Fatalf("Update error = %v, want restricted", err)
	}
	if fakeOrderRepository.received != nil {
		t.Fatal("restricted update must not reach repository")
	}
}
