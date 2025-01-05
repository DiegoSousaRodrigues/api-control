package repository

import (
	"log"

	"github.com/api-control/internal/domain"
)

var SkuRepository ISkuRepository = &skuRepository{}

type ISkuRepository interface {
	List() (entity *[]domain.Sku, err error)
	Add(entity domain.Sku) (err error)
}

type skuRepository struct {
	db domain.BaseRepository
}

func (c *skuRepository) List() (entity *[]domain.Sku, err error) {
	db := c.db.PSQL()

	if err := db.Order("id").Find(&entity); err.Error != nil {
		log.Fatalf("Erro ao buscar produtos: %v", err)
		return nil, err.Error
	}

	if entity == nil {
		return nil, err
	}

	return entity, nil
}

func (c *skuRepository) Add(client domain.Sku) (err error) {
	db := c.db.PSQL()

	if err := db.Create(&client); err.Error != nil {
		return err.Error
	}

	return nil
}
