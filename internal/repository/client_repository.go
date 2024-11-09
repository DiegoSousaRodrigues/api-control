package repository

import (
	"github.com/autorei/api-control/internal/domain"
	"log"
)

var ClientRepository IClientRepository = &clientRepository{}

type IClientRepository interface {
	List() (entity *[]domain.Client, err error)
}

type clientRepository struct{
	db domain.BaseRepository
}

func (c *clientRepository) List() (entity *[]domain.Client, err error) {
	db := c.db.PSQL()

	if err := db.Find(&entity); err.Error != nil {
		log.Fatalf("Erro ao buscar clientes: %v", err)
		return nil, err.Error
	}

	if entity == nil {
		return nil, err
	}

	return entity, nil
}