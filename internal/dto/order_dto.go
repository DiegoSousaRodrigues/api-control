package dto

import (
	"time"

	"github.com/api-control/internal/domain"
)

type (
	OrderSkuDTO struct {
		SkuID    int64 `json:"skuId"`
		Quantity int   `json:"quantity"`
	}

	OrderRequestDTO struct {
		ClientID    int64         `json:"clientId"`
		Observation string        `json:"observation"`
		OrderSkus   []OrderSkuDTO `json:"orderSkus"`
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
	return &domain.Order{
		ClientId:    dto.ClientID,
		Observation: dto.Observation,
	}, nil
}