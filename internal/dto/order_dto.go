package dto

import (
	"fmt"
	"strconv"
	"time"

	"github.com/api-control/internal/domain"
	"github.com/api-control/internal/repository"
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
		ID          int64     `json:"id"`
		DateCreated time.Time `json:"dateCreated"`
		LastUpdated time.Time `json:"lastUpdated"`
		Observation string    `json:"observation"`
		Client      ClientDTO `json:"client"`
		OrderSkus   []SkuDTO  `json:"orderSkus"`
		PriceTotal  string    `json:"priceTotal"`
	}
)

func ParseOrderToDTO(entity domain.Order) OrderResponseDTO {
	var skusDTO []SkuDTO
	var total float64

	for _, orderSku := range entity.OrderSkus {
		skusDTO = append(skusDTO, ParseSkuToDTO(orderSku.Sku))
		total += orderSku.Price
	}

	clientDTO := ParseClientToDTO(entity.Client)

	return OrderResponseDTO{
		ID:          entity.ID,
		DateCreated: entity.DateCreated,
		LastUpdated: entity.LastUpdated,
		Observation: entity.Observation,
		Client:      clientDTO,
		OrderSkus:   skusDTO,
		PriceTotal:  utils.Float64ToCurrency(total),
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

	for i, v := range *orderSkus {
		sku, err := repository.SkuRepository.FindByID(fmt.Sprint(v.SkuID))
		if err != nil {
			return nil, err
		}

		(*orderSkus)[i].Price = float64(v.Quantity) * sku.Price
		(*orderSkus)[i].Name = sku.Name
	}

	return &domain.Order{
		ClientId:    int64(clientID),
		Observation: dto.Observation,
		OrderSkus:   *orderSkus,
	}, nil
}

func ParseOrderSkuRequestToEntity(dto []OrderSkuDTO) (*[]domain.OrderSku, error) {
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
