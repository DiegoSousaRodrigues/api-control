package repository

import (
	"log"

	"github.com/api-control/internal/domain"
)

var SkuRepository ISkuRepository = &skuRepository{}

type ISkuRepository interface {
	List() (entity *[]domain.Sku, err error)
	Add(entity domain.Sku) (err error)
	ChangeStatus(id int64, status bool) (err error)
	FindByID(id string) (entity *domain.Sku, err error)
	Update(id int64, entity domain.Sku) (err error)
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

func (c *skuRepository) ChangeStatus(id int64, status bool) (err error) {
	db := c.db.PSQL()

	sql := "update sku set active = ? where id = ?"
	if err := db.Exec(sql, status, id); err.Error != nil {
		return err.Error
	}

	return nil
}

func (c *skuRepository) FindByID(id string) (entity *domain.Sku, err error) {
	db := c.db.PSQL()

	if err := db.Where("id = ?", id).First(&entity); err.Error != nil {
		return nil, err.Error
	}

	return entity, nil
}

func (c *skuRepository) Update(id int64, entity domain.Sku) (err error) {
	db := c.db.PSQL()
	entity.ID = id

	if err := db.Save(&entity); err.Error != nil {
		return err.Error
	}

	return nil
}