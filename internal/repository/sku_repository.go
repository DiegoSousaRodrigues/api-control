package repository

import (
	"time"

	"github.com/api-control/internal/domain"
	"gorm.io/gorm"
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

	result := db.Model(&domain.Sku{}).Where("id = ?", id).Update("active", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
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

	result := db.Model(&domain.Sku{}).Where("id = ?", id).Updates(skuUpdateFields(entity))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func skuUpdateFields(entity domain.Sku) map[string]interface{} {
	fields := map[string]interface{}{
		"last_updated":   time.Now(),
		"name":           entity.Name,
		"price":          entity.Price,
		"purchase_price": entity.PurchasePrice,
		"sale_price":     entity.SalePrice,
	}

	if entity.ImageUrl != nil {
		fields["image_url"] = entity.ImageUrl
	}

	return fields
}
