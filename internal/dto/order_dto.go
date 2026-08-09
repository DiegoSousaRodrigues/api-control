package dto

import (
	"errors"
	"strconv"
	"time"

	"github.com/api-control/internal/domain"
	"github.com/api-control/internal/utils"
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
		PriceTotal  string                 `json:"priceTotal"`
	}

	OrderItemResponseDTO struct {
		ID        int64  `json:"id"`
		SkuID     int64  `json:"skuId"`
		Name      string `json:"name"`
		Quantity  int    `json:"quantity"`
		UnitPrice string `json:"unitPrice"`
		LineTotal string `json:"lineTotal"`
		Sku       SkuDTO `json:"sku"`
	}
)

var (
	ErrOrderRequiresProduct         = errors.New("order requires at least one product")
	ErrOrderProductQuantityPositive = errors.New("order product quantity must be greater than zero")
	ErrOrderProductDuplicated       = errors.New("order contains duplicated product")
)

func ParseOrderToDTO(entity domain.Order) OrderResponseDTO {
	var orderSkusDTO []OrderItemResponseDTO
	var total float64

	for _, orderSku := range entity.OrderSkus {
		orderSkusDTO = append(orderSkusDTO, ParseOrderItemToDTO(orderSku))
		total += orderSku.Price
	}

	clientDTO := ParseClientToDTO(entity.Client)

	return OrderResponseDTO{
		ID:          entity.ID,
		DateCreated: entity.DateCreated,
		LastUpdated: entity.LastUpdated,
		Observation: entity.Observation,
		Client:      clientDTO,
		OrderSkus:   orderSkusDTO,
		PriceTotal:  utils.Float64ToCurrency(total),
	}
}

func ParseOrderItemToDTO(entity domain.OrderSku) OrderItemResponseDTO {
	unitPrice := float64(0)
	if entity.Quantity > 0 {
		unitPrice = entity.Price / float64(entity.Quantity)
	}

	return OrderItemResponseDTO{
		ID:        entity.ID,
		SkuID:     entity.SkuID,
		Name:      entity.Name,
		Quantity:  entity.Quantity,
		UnitPrice: utils.Float64ToCurrency(unitPrice),
		LineTotal: utils.Float64ToCurrency(entity.Price),
		Sku:       ParseSkuToDTO(entity.Sku),
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
