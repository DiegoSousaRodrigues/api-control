package dto

import (
	"errors"
	"strconv"
	"time"

	"github.com/api-control/internal/domain"
	"github.com/shopspring/decimal"
)

type (
	OrderSkuDTO struct {
		ProductId string `json:"productId"`
		Quantity  string `json:"quantity"`
	}

	OrderRequestDTO struct {
		ClientID    string        `json:"clientId"`
		Observation string        `json:"observation"`
		Products    []OrderSkuDTO `json:"products"`
	}

	OrderResponseDTO struct {
		ID          int64                  `json:"id"`
		DateCreated time.Time              `json:"dateCreated"`
		LastUpdated time.Time              `json:"lastUpdated"`
		Observation string                 `json:"observation"`
		Client      ClientDTO              `json:"client"`
		OrderSkus   []OrderItemResponseDTO `json:"orderSkus"`
		PriceTotal  Money                  `json:"priceTotal"`
	}

	OrderItemResponseDTO struct {
		ID        int64              `json:"id"`
		SkuID     int64              `json:"skuId"`
		Name      string             `json:"name"`
		Quantity  int                `json:"quantity"`
		UnitPrice Money              `json:"unitPrice"`
		LineTotal Money              `json:"lineTotal"`
		Sku       OrderSkuSummaryDTO `json:"sku"`
	}

	OrderSkuSummaryDTO struct {
		ID        int64   `json:"id"`
		Name      string  `json:"name"`
		SalePrice Money   `json:"salePrice"`
		Active    bool    `json:"active"`
		ImageUrl  *string `json:"imageUrl,omitempty"`
	}
)

var (
	ErrOrderRequiresProduct         = errors.New("order requires at least one product")
	ErrOrderProductQuantityPositive = errors.New("order product quantity must be greater than zero")
	ErrOrderProductDuplicated       = errors.New("order contains duplicated product")
)

func ParseOrderToDTO(entity domain.Order) OrderResponseDTO {
	var orderSkusDTO []OrderItemResponseDTO
	total := decimal.Zero

	for _, orderSku := range entity.OrderSkus {
		orderSkusDTO = append(orderSkusDTO, ParseOrderItemToDTO(orderSku))
		total = total.Add(orderSku.Price)
	}

	clientDTO := ParseClientToDTO(entity.Client)

	return OrderResponseDTO{
		ID:          entity.ID,
		DateCreated: entity.DateCreated,
		LastUpdated: entity.LastUpdated,
		Observation: entity.Observation,
		Client:      clientDTO,
		OrderSkus:   orderSkusDTO,
		PriceTotal:  NewMoney(total),
	}
}

func ParseOrderItemToDTO(entity domain.OrderSku) OrderItemResponseDTO {
	unitPrice := decimal.Zero
	if entity.Quantity > 0 {
		unitPrice = entity.Price.Div(decimal.NewFromInt(int64(entity.Quantity))).Round(2)
	}

	return OrderItemResponseDTO{
		ID:        entity.ID,
		SkuID:     entity.SkuID,
		Name:      entity.Name,
		Quantity:  entity.Quantity,
		UnitPrice: NewMoney(unitPrice),
		LineTotal: NewMoney(entity.Price),
		Sku: OrderSkuSummaryDTO{
			ID:        entity.Sku.ID,
			Name:      entity.Sku.Name,
			SalePrice: NewMoney(entity.Sku.SalePrice),
			Active:    entity.Sku.Active,
			ImageUrl:  entity.Sku.ImageUrl,
		},
	}
}

func ParseOrderRequestToEntity(dto OrderRequestDTO) (*domain.Order, error) {
	clientID, err := strconv.Atoi(dto.ClientID)
	if err != nil {
		return nil, err
	}

	orderSkus, err := ParseOrderSkuRequestToEntity(dto.Products)
	if err != nil {
		return nil, err
	}

	return &domain.Order{
		ClientId:    int64(clientID),
		Observation: dto.Observation,
		OrderSkus:   *orderSkus,
	}, nil
}

func ParseOrderSkuRequestToEntity(dto []OrderSkuDTO) (*[]domain.OrderSku, error) {
	var list []domain.OrderSku
	seenProducts := map[int64]bool{}

	if len(dto) == 0 {
		return nil, ErrOrderRequiresProduct
	}

	for _, v := range dto {
		productID, err := strconv.ParseInt(v.ProductId, 10, 64)
		if err != nil {
			return nil, err
		}

		quantity, err := strconv.Atoi(v.Quantity)
		if err != nil {
			return nil, err
		}
		if quantity <= 0 {
			return nil, ErrOrderProductQuantityPositive
		}
		if seenProducts[productID] {
			return nil, ErrOrderProductDuplicated
		}
		seenProducts[productID] = true

		orderSku := domain.OrderSku{
			SkuID:    productID,
			Quantity: quantity,
		}
		list = append(list, orderSku)
	}

	return &list, nil

}
