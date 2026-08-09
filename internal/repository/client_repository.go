package repository

import (
	"time"

	"github.com/api-control/internal/domain"
	"gorm.io/gorm"
)

var ClientRepository IClientRepository = &clientRepository{}

type IClientRepository interface {
	List() (entity *[]domain.Client, err error)
	Add(entity domain.Client) (err error)
	FindByID(id string) (entity *domain.Client, err error)
	Update(id int64, entity domain.Client) (err error)
	ChangeStatus(id int64, status bool) (err error)
}

type clientRepository struct {
	db domain.BaseRepository
}

func (c *clientRepository) ChangeStatus(id int64, status bool) (err error) {
	db := c.db.PSQL()

	result := db.Model(&domain.Client{}).Where("id = ?", id).Update("active", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (c *clientRepository) Update(id int64, entity domain.Client) (err error) {
	db := c.db.PSQL()

	result := db.Model(&domain.Client{}).Where("id = ?", id).Updates(clientUpdateFields(entity))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func clientUpdateFields(entity domain.Client) map[string]interface{} {
	return map[string]interface{}{
		"last_updated":      time.Now(),
		"name":              entity.Name,
		"document":          entity.Document,
		"phone":             entity.Phone,
		"telephone":         entity.Telephone,
		"birthdate":         entity.Birthdate,
		"street":            entity.Street,
		"quarter":           entity.Quarter,
		"number":            entity.Number,
		"complement":        entity.Complement,
		"zipcode":           entity.Zipcode,
		"address_type":      entity.AddressType,
		"address_reference": entity.AddressReference,
		"position":          entity.Position,
	}
}

func (c *clientRepository) FindByID(id string) (entity *domain.Client, err error) {
	db := c.db.PSQL()

	if err := db.Where("id = ?", id).First(&entity); err.Error != nil {
		return nil, err.Error
	}

	return entity, nil
}

func (c *clientRepository) Add(client domain.Client) (err error) {
	db := c.db.PSQL()

	if err := db.Create(&client); err.Error != nil {
		return err.Error
	}

	return nil
}

func (c *clientRepository) List() (entity *[]domain.Client, err error) {
	db := c.db.PSQL()

	if err := db.Order("id").Find(&entity); err.Error != nil {
		return nil, err.Error
	}

	if entity == nil {
		return nil, err
	}

	return entity, nil
}
