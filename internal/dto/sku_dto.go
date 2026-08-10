package dto

import (
	"errors"
	"mime/multipart"

	"github.com/api-control/internal/domain"
)

type (
	SkuProductRequest struct {
		Name          string `json:"name"`
		PurchasePrice *Money `json:"purchasePrice"`
		SalePrice     *Money `json:"salePrice"`
	}

	SkuDTO struct {
		ID            int64   `json:"id"`
		Name          string  `json:"name"`
		PurchasePrice *Money  `json:"purchasePrice"`
		SalePrice     Money   `json:"salePrice"`
		Active        bool    `json:"active"`
		ImageUrl      *string `json:"imageUrl,omitempty"`
	}
)

var (
	ErrSkuNameRequired          = errors.New("sku name is required")
	ErrSkuPurchasePriceRequired = errors.New("sku purchase price is required")
	ErrSkuSalePriceRequired     = errors.New("sku sale price is required")
	ErrSkuPurchasePriceNegative = errors.New("sku purchase price cannot be negative")
	ErrSkuSalePriceNotPositive  = errors.New("sku sale price must be greater than zero")
)

func (request SkuProductRequest) Validate() error {
	if request.Name == "" {
		return ErrSkuNameRequired
	}
	if request.PurchasePrice == nil {
		return ErrSkuPurchasePriceRequired
	}
	if request.SalePrice == nil {
		return ErrSkuSalePriceRequired
	}
	if request.PurchasePrice.Decimal().IsNegative() {
		return ErrSkuPurchasePriceNegative
	}
	if !request.SalePrice.Decimal().IsPositive() {
		return ErrSkuSalePriceNotPositive
	}
	return nil
}

func ParseSkuToDTO(entity domain.Sku) SkuDTO {
	var purchasePrice *Money
	if entity.PurchasePrice != nil {
		value := NewMoney(*entity.PurchasePrice)
		purchasePrice = &value
	}

	return SkuDTO{
		ID:            entity.ID,
		Name:          entity.Name,
		PurchasePrice: purchasePrice,
		SalePrice:     NewMoney(entity.SalePrice),
		Active:        entity.Active,
		ImageUrl:      entity.ImageUrl,
	}
}

func ParseSkuRequestToEntity(request SkuProductRequest, imageUrl *string) (*domain.Sku, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	purchasePrice := request.PurchasePrice.Decimal()
	salePrice := request.SalePrice.Decimal()
	legacyPrice, _ := salePrice.Float64()

	return &domain.Sku{
		Name:          request.Name,
		Price:         legacyPrice,
		PurchasePrice: &purchasePrice,
		SalePrice:     salePrice,
		ImageUrl:      imageUrl,
	}, nil
}

// Kept here because the multipart boundary belongs to the HTTP request, not
// to the JSON product contract.
type SkuUpload struct {
	Product SkuProductRequest
	File    *multipart.FileHeader
}
