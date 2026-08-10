package dto

import (
	"errors"
	"time"

	"github.com/api-control/internal/domain"
	"github.com/shopspring/decimal"
)

type (
	OrderSkuDTO struct {
		ProductId int64 `json:"productId"`
		Quantity  int   `json:"quantity"`
	}

	OrderRequestDTO struct {
		ClientID             int64         `json:"clientId"`
		OrderYear            int16         `json:"orderYear"`
		OrderMonth           int16         `json:"orderMonth"`
		PreviousMonthPayment *Money        `json:"previousMonthPayment"`
		Observation          string        `json:"observation"`
		Products             []OrderSkuDTO `json:"products"`
	}

	OrderResponseDTO struct {
		ID                   int64                  `json:"id"`
		DateCreated          time.Time              `json:"dateCreated"`
		LastUpdated          time.Time              `json:"lastUpdated"`
		Observation          string                 `json:"observation"`
		Client               ClientDTO              `json:"client"`
		OrderSkus            []OrderItemResponseDTO `json:"orderSkus"`
		PriceTotal           Money                  `json:"priceTotal"`
		OrderYear            *int16                 `json:"orderYear"`
		OrderMonth           *int16                 `json:"orderMonth"`
		OpeningBalance       Money                  `json:"openingBalance"`
		PreviousMonthPayment Money                  `json:"previousMonthPayment"`
		CarriedBalance       Money                  `json:"carriedBalance"`
		AmountDue            Money                  `json:"amountDue"`
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

	OpenBalanceDTO struct {
		ClientID   int64 `json:"clientId"`
		OrderYear  int16 `json:"orderYear"`
		OrderMonth int16 `json:"orderMonth"`
		Balance    Money `json:"balance"`
	}
)

var (
	ErrOrderRequiresProduct         = errors.New("order requires at least one product")
	ErrOrderProductQuantityPositive = errors.New("order product quantity must be greater than zero")
	ErrOrderProductDuplicated       = errors.New("order contains duplicated product")
	ErrOrderClientRequired          = errors.New("order client is required")
	ErrOrderYearInvalid             = errors.New("order year is invalid")
	ErrOrderMonthInvalid            = errors.New("order month is invalid")
	ErrOrderPaymentRequired         = errors.New("previous month payment is required")
	ErrOrderPaymentNegative         = errors.New("previous month payment cannot be negative")
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
		ID:                   entity.ID,
		DateCreated:          entity.DateCreated,
		LastUpdated:          entity.LastUpdated,
		Observation:          entity.Observation,
		Client:               clientDTO,
		OrderSkus:            orderSkusDTO,
		PriceTotal:           NewMoney(total),
		OrderYear:            entity.OrderYear,
		OrderMonth:           entity.OrderMonth,
		OpeningBalance:       NewMoney(entity.OpeningBalance),
		PreviousMonthPayment: NewMoney(entity.PreviousMonthPayment),
		CarriedBalance:       NewMoney(entity.CarriedBalance),
		AmountDue:            NewMoney(entity.CarriedBalance.Add(total)),
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
	if dto.ClientID <= 0 {
		return nil, ErrOrderClientRequired
	}
	if dto.OrderYear < 1 {
		return nil, ErrOrderYearInvalid
	}
	if dto.OrderMonth < 1 || dto.OrderMonth > 12 {
		return nil, ErrOrderMonthInvalid
	}
	if dto.PreviousMonthPayment == nil {
		return nil, ErrOrderPaymentRequired
	}
	if dto.PreviousMonthPayment.Decimal().IsNegative() {
		return nil, ErrOrderPaymentNegative
	}

	orderSkus, err := ParseOrderSkuRequestToEntity(dto.Products)
	if err != nil {
		return nil, err
	}

	return &domain.Order{
		ClientId:             dto.ClientID,
		OrderYear:            &dto.OrderYear,
		OrderMonth:           &dto.OrderMonth,
		PreviousMonthPayment: dto.PreviousMonthPayment.Decimal(),
		Observation:          dto.Observation,
		OrderSkus:            *orderSkus,
	}, nil
}

func ParseOrderSkuRequestToEntity(dto []OrderSkuDTO) (*[]domain.OrderSku, error) {
	var list []domain.OrderSku
	seenProducts := map[int64]bool{}

	if len(dto) == 0 {
		return nil, ErrOrderRequiresProduct
	}

	for _, v := range dto {
		if v.ProductId <= 0 {
			return nil, errors.New("order product id must be positive")
		}
		if v.Quantity <= 0 {
			return nil, ErrOrderProductQuantityPositive
		}
		if seenProducts[v.ProductId] {
			return nil, ErrOrderProductDuplicated
		}
		seenProducts[v.ProductId] = true

		orderSku := domain.OrderSku{
			SkuID:    v.ProductId,
			Quantity: v.Quantity,
		}
		list = append(list, orderSku)
	}

	return &list, nil

}
