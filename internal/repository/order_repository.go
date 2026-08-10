package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/api-control/internal/domain"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

var OrderRepository IOrderRepository = &orderRepository{}

type IOrderRepository interface {
	List() (entity *[]domain.Order, err error)
	Add(entity domain.Order) (err error)
	FindByID(id string) (entity *domain.Order, err error)
	Update(id int64, entity domain.Order) (err error)
}

type orderRepository struct {
	db domain.BaseRepository
}

var (
	ErrOrderClientInactive = errors.New("order client is inactive")
	ErrOrderSkuInactive    = errors.New("order sku is inactive")
)

func (c *orderRepository) List() (entity *[]domain.Order, err error) {
	db := c.db.PSQL()

	if err := db.Order("id").Preload("Client").Preload("OrderSkus").Preload("OrderSkus.Sku").Find(&entity); err.Error != nil {
		return nil, err.Error
	}

	if entity == nil {
		return nil, err
	}

	return entity, nil
}

func (c *orderRepository) Add(order domain.Order) (err error) {
	db := c.db.PSQL()

	return db.Transaction(func(tx *gorm.DB) error {
		if err := validateOrderClient(tx, order.ClientId); err != nil {
			return err
		}
		if err := hydrateOrderSkuSnapshots(tx, &order); err != nil {
			return err
		}

		return tx.Create(&order).Error
	})
}

func (c *orderRepository) FindByID(id string) (entity *domain.Order, err error) {
	db := c.db.PSQL()
	var order domain.Order

	if err := db.Where("id = ?", id).Preload("Client").Preload("OrderSkus").Preload("OrderSkus.Sku").First(&order); err.Error != nil {
		return nil, err.Error
	}

	return &order, nil
}

func (c *orderRepository) Update(id int64, entity domain.Order) (err error) {
	db := c.db.PSQL()

	return db.Transaction(func(tx *gorm.DB) error {
		if err := validateOrderClient(tx, entity.ClientId); err != nil {
			return err
		}
		if err := hydrateOrderSkuSnapshots(tx, &entity); err != nil {
			return err
		}

		result := tx.Model(&domain.Order{}).Where("id = ?", id).Updates(orderUpdateFields(entity))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Where("order_id = ?", id).Delete(&domain.OrderSku{}).Error; err != nil {
			return err
		}
		if len(entity.OrderSkus) > 0 {
			for index := range entity.OrderSkus {
				entity.OrderSkus[index].OrderID = id
			}
			if err := tx.Create(&entity.OrderSkus).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func orderUpdateFields(entity domain.Order) map[string]interface{} {
	return map[string]interface{}{
		"last_updated": time.Now(),
		"client_id":    entity.ClientId,
		"observation":  entity.Observation,
	}
}

func hydrateOrderSkuSnapshots(tx *gorm.DB, order *domain.Order) error {
	for index := range order.OrderSkus {
		var sku domain.Sku
		if err := tx.Where("id = ?", order.OrderSkus[index].SkuID).First(&sku).Error; err != nil {
			return err
		}
		if !sku.Active {
			return fmt.Errorf("%w: %d", ErrOrderSkuInactive, sku.ID)
		}

		applyOrderSkuSnapshot(&order.OrderSkus[index], sku)
	}

	return nil
}

func validateOrderClient(tx *gorm.DB, clientID int64) error {
	var client domain.Client
	if err := tx.Where("id = ?", clientID).First(&client).Error; err != nil {
		return err
	}
	if !client.Active {
		return fmt.Errorf("%w: %d", ErrOrderClientInactive, client.ID)
	}

	return nil
}

func applyOrderSkuSnapshot(orderSku *domain.OrderSku, sku domain.Sku) {
	orderSku.Name = sku.Name
	orderSku.Price = sku.SalePrice.Mul(decimal.NewFromInt(int64(orderSku.Quantity)))
}
