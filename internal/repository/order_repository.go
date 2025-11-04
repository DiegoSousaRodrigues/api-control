package repository

import (
	"log"

	"github.com/api-control/internal/domain"
)

var OrderRepository IOrderRepository = &orderRepository{}

type IOrderRepository interface {
	List() (entity *[]domain.Order, err error)
	Add(entity domain.Order) (err error)
	ChangeStatus(id int64, status bool) (err error)
	FindByID(id string) (entity *domain.Order, err error)
	Update(id int64, entity domain.Order) (err error)
}

type orderRepository struct {
	db domain.BaseRepository
}

func (c *orderRepository) List() (entity *[]domain.Order, err error) {
	db := c.db.PSQL()

	if err := db.Order("id").Preload("Client").Preload("OrderSkus").Find(&entity); err.Error != nil {
		log.Fatalf("Erro ao buscar produtos: %v", err)
		return nil, err.Error
	}

	if entity == nil {
		return nil, err
	}

	return entity, nil
}

func (c *orderRepository) Add(client domain.Order) (err error) {
	db := c.db.PSQL()

	if err := db.Create(&client); err.Error != nil {
		return err.Error
	}

	return nil
}

func (c *orderRepository) ChangeStatus(id int64, status bool) (err error) {
	db := c.db.PSQL()

	sql := "update sku set active = ? where id = ?"
	if err := db.Exec(sql, status, id); err.Error != nil {
		return err.Error
	}

	return nil
}

func (c *orderRepository) FindByID(id string) (entity *domain.Order, err error) {
	db := c.db.PSQL()

	if err := db.Where("id = ?", id).First(&entity); err.Error != nil {
		return nil, err.Error
	}

	return entity, nil
}

func (c *orderRepository) Update(id int64, entity domain.Order) (err error) {
	db := c.db.PSQL()
	entity.ID = id

	if err := db.Save(&entity); err.Error != nil {
		return err.Error
	}

	return nil
}
