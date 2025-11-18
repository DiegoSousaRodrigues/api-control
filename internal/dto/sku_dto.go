package dto

import (
	"mime/multipart"

	"github.com/api-control/internal/domain"
	"github.com/api-control/internal/utils"
)

type (
	SkuDTO struct {
		ID     int64                 `form:"id" json:"id"`
		Name   string                `form:"name" json:"name" binding:"required"`
		Price  string                `form:"price" json:"price" binding:"required"`
		File   *multipart.FileHeader `form:"file"`
		Active bool                  `form:"active" json:"active" gorm:"not null;default:true"`
	}
)

func ParseSkuToDTO(entity domain.Sku) SkuDTO {
	price := utils.Float64ToCurrency(entity.Price)

	return SkuDTO{
		ID:     entity.ID,
		Name:   entity.Name,
		Price:  price,
		Active: entity.Active,
	}
}

func ParseSkuRequestToEntity(dto SkuDTO) (*domain.Sku, error) {
	price, err := utils.CurrencyToFloat64(dto.Price)

	if err != nil {
		return nil, err
	}

	return &domain.Sku{
		Name:   dto.Name,
		Price:  price,
		Active: dto.Active,
	}, nil
}
