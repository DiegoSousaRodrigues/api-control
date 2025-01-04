package dto

import (
	"github.com/autorei/api-control/internal/domain"
	"github.com/autorei/api-control/internal/utils"
)

type (
	SkuDTO struct {
		ID          int64   	`json:"id"`
		Name        string      `json:"name"     gorm:"not null"`
		Price       string      `json:"price"    gorm:"not null"`
		Active      bool        `json:"active"   gorm:"not null;default:true"`
	}
)

func ParseSkuToDTO(entity domain.Sku) SkuDTO {
	price := utils.Float64ToCurrency(entity.Price)

	return SkuDTO{
		ID:               entity.ID,
		Name:             entity.Name,
		Price: 			  price,
		Active: 		  entity.Active,
	}
}

func ParseSkuRequestToEntity(dto SkuDTO) (*domain.Sku, error) {
	price, err := utils.CurrencyToFloat64(dto.Price)

	if err != nil {
		return nil, err
	}

	return &domain.Sku{
		Name:         dto.Name,
		Price: 		  price,
		Active: 	  dto.Active,
	}, nil
}