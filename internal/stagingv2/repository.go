package stagingv2

import (
	"context"
	"errors"
	"strings"
	"time"

	domainv2 "github.com/api-control/internal/domain/v2"
	"gorm.io/gorm"
)

var ErrInvalidIdentifier = errors.New("identifier must be positive")

// Repository targets only the plural v2 staging tables. It is intentionally
// not assigned to any legacy runtime singleton during phase 1.
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) (*Repository, error) {
	if db == nil {
		return nil, errors.New("staging v2 database is required")
	}
	return &Repository{db: db}, nil
}

func (repository *Repository) FindActiveUserByLogin(ctx context.Context, login string) (*domainv2.User, error) {
	var user domainv2.User
	err := repository.db.WithContext(ctx).
		Where("login = ? AND active = TRUE", normalizeLogin(login)).
		Take(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (repository *Repository) LoginExists(ctx context.Context, login string) (bool, error) {
	var count int64
	err := repository.db.WithContext(ctx).Model(&domainv2.User{}).
		Where("login = ?", normalizeLogin(login)).Count(&count).Error
	return count > 0, err
}

func (repository *Repository) CreateInitialUser(ctx context.Context, login, passwordHash string) error {
	normalized := normalizeLogin(login)
	return repository.db.WithContext(ctx).Create(&domainv2.User{
		Name:         normalized,
		Login:        normalized,
		PasswordHash: passwordHash,
		Active:       true,
	}).Error
}

func (repository *Repository) CreateClient(ctx context.Context, client *domainv2.Client) error {
	if client == nil {
		return errors.New("client is required")
	}
	client.Document = normalizeDocument(client.Document)
	return repository.db.WithContext(ctx).Create(client).Error
}

func (repository *Repository) FindClient(ctx context.Context, id int64) (*domainv2.Client, error) {
	if id <= 0 {
		return nil, ErrInvalidIdentifier
	}
	var client domainv2.Client
	if err := repository.db.WithContext(ctx).Take(&client, id).Error; err != nil {
		return nil, err
	}
	return &client, nil
}

func (repository *Repository) ListClients(ctx context.Context) ([]domainv2.Client, error) {
	clients := make([]domainv2.Client, 0)
	err := repository.db.WithContext(ctx).Order("id").Find(&clients).Error
	return clients, err
}

func (repository *Repository) UpdateClient(ctx context.Context, id int64, client domainv2.Client) error {
	if id <= 0 {
		return ErrInvalidIdentifier
	}
	result := repository.db.WithContext(ctx).Model(&domainv2.Client{}).Where("id = ?", id).Updates(map[string]any{
		"name":              client.Name,
		"document":          normalizeDocument(client.Document),
		"birth_date":        client.BirthDate,
		"phone":             client.Phone,
		"secondary_phone":   client.SecondaryPhone,
		"street":            client.Street,
		"neighborhood":      client.Neighborhood,
		"address_number":    client.AddressNumber,
		"complement":        client.Complement,
		"postal_code":       client.PostalCode,
		"address_type":      client.AddressType,
		"address_reference": client.AddressReference,
		"position":          client.Position,
		"updated_at":        time.Now(),
	})
	return affected(result)
}

func (repository *Repository) SetClientActive(ctx context.Context, id int64, active bool) error {
	if id <= 0 {
		return ErrInvalidIdentifier
	}
	return affected(repository.db.WithContext(ctx).Model(&domainv2.Client{}).Where("id = ?", id).Updates(map[string]any{
		"active": active, "updated_at": time.Now(),
	}))
}

func (repository *Repository) CreateProduct(ctx context.Context, product *domainv2.Product) error {
	if product == nil {
		return errors.New("product is required")
	}
	return repository.db.WithContext(ctx).Create(product).Error
}

func (repository *Repository) FindProduct(ctx context.Context, id int64) (*domainv2.Product, error) {
	if id <= 0 {
		return nil, ErrInvalidIdentifier
	}
	var product domainv2.Product
	if err := repository.db.WithContext(ctx).Take(&product, id).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

func (repository *Repository) ListProducts(ctx context.Context) ([]domainv2.Product, error) {
	products := make([]domainv2.Product, 0)
	err := repository.db.WithContext(ctx).Order("id").Find(&products).Error
	return products, err
}

func (repository *Repository) UpdateProduct(ctx context.Context, id int64, product domainv2.Product) error {
	if id <= 0 {
		return ErrInvalidIdentifier
	}
	result := repository.db.WithContext(ctx).Model(&domainv2.Product{}).Where("id = ?", id).Updates(map[string]any{
		"name":           product.Name,
		"purchase_price": product.PurchasePrice,
		"sale_price":     product.SalePrice,
		"image_url":      product.ImageURL,
		"updated_at":     time.Now(),
	})
	return affected(result)
}

func (repository *Repository) SetProductActive(ctx context.Context, id int64, active bool) error {
	if id <= 0 {
		return ErrInvalidIdentifier
	}
	return affected(repository.db.WithContext(ctx).Model(&domainv2.Product{}).Where("id = ?", id).Updates(map[string]any{
		"active": active, "updated_at": time.Now(),
	}))
}

func affected(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func normalizeLogin(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeDocument(value string) string {
	var result strings.Builder
	for _, character := range value {
		if character >= '0' && character <= '9' {
			result.WriteRune(character)
		}
	}
	return result.String()
}
