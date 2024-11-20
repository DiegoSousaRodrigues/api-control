package repository

import (
	"github.com/autorei/api-control/internal/domain"
	"log"
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

	sql := "update client set active = ? where id = ?"
	if err := db.Exec(sql, status, id); err.Error != nil {
		return err.Error
	}

	return nil
}

func (c *clientRepository) Update(id int64, entity domain.Client) (err error) {
	db := c.db.PSQL()
	entity.ID = id

	if err := db.Save(&entity); err.Error != nil {
		return err.Error
	}

	return nil
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
		log.Fatalf("Erro ao buscar clientes: %v", err)
		return nil, err.Error
	}

	if entity == nil {
		return nil, err
	}

	return entity, nil
}
