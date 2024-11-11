package service

import (
	"github.com/autorei/api-control/internal/dto"
	"github.com/autorei/api-control/internal/repository"
)

var ClientService IClientService = &clientService{}

type IClientService interface {
	List() (*[]dto.ClientDTO, error)
}

type clientService struct{}

func (c *clientService) List() (*[]dto.ClientDTO, error) {
	listEntity, err := repository.ClientRepository.List()
	if err != nil {
		return nil, err
	}

	var listDTO []dto.ClientDTO

	for _, value := range *listEntity {
		listDTO = append(listDTO, dto.ParseToDTO(value))
	}

	return &listDTO, nil
}
