package dto

import (
	"strconv"
	"time"

	"github.com/api-control/internal/domain"
)

type (
	OrderSkuDTO struct {
		ProductId string `json:"productId"`
		Quantity  string `json:"quantity"`
	}

	OrderRequestDTO struct {
		ClientID    string         `json:"clientId"`
		Observation string        `json:"observation"`
		Products    []OrderSkuDTO `json:"products"`
	} 

	OrderResponseDTO struct {
		ID          int64      `json:"id"`
		DateCreated time.Time  `json:"dateCreated"`
		LastUpdated time.Time  `json:"lastUpdated"`
		Observation string     `json:"observation"`
		Client      ClientDTO  `json:"client"`
		OrderSkus   []SkuDTO   `json:"orderSkus"`
		Total       string     `json:"total"`
	}
)

func ParseOrderToDTO(entity domain.Order) OrderResponseDTO {
	var skusDTO []SkuDTO
	var total float64

	for _, orderSku := range entity.OrderSkus {
		skusDTO = append(skusDTO, ParseSkuToDTO(orderSku.Sku))
		total += orderSku.Price * float64(orderSku.Quantity)
	}

	clientDTO := ParseClientToDTO(entity.Client)

	return OrderResponseDTO{
		ID:          entity.ID,
		DateCreated: entity.DateCreated,
		LastUpdated: entity.LastUpdated,
		Observation: entity.Observation,
		Client:      clientDTO,
		OrderSkus:   skusDTO,
		// Total:       utils.Float64ToCurrency(total), // You might need a currency utility function
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
		OrderSkus: *orderSkus,
	}, nil
}

func ParseOrderSkuRequestToEntity (dto []OrderSkuDTO) (*[]domain.OrderSku, error) {
	var list []domain.OrderSku

	for _, v := range dto {
		productID, err := strconv.ParseInt(v.ProductId, 10, 64)
		if err != nil {
			return nil, err
		}

		quantity, err := strconv.Atoi(v.Quantity)
		if err != nil {
			return nil, err
		}

		orderSku := domain.OrderSku{
			SkuID:    productID,
			Quantity: quantity,
		}
		list = append(list, orderSku)
	}

	return &list, nil
	
}