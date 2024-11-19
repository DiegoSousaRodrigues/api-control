package service

import (
	"github.com/autorei/api-control/internal/dto"
	"github.com/autorei/api-control/internal/repository"
)

var ClientService IClientService = &clientService{}

type IClientService interface {
	List() (*[]dto.ClientDTO, error)
	Add(dto.ClientRequest) (err error)
	FindByID(id string) (*dto.ClientDTO, error)
}

type clientService struct{}

func (c *clientService) FindByID(id string) (*dto.ClientDTO, error) {
	entity, err := repository.ClientRepository.FindByID(id)
	if err != nil {
		return nil, err
	}

	dtoClient := dto.ParseToDTO(*entity)
	return &dtoClient, nil
}

func (c *clientService) Add(clientDTO dto.ClientRequest) (err error) {
	entity, err := dto.ParseToEntity(clientDTO)
	if err != nil {
		return err
	}

	err = repository.ClientRepository.Add(*entity)
	if err != nil {
		return err
	}

	return nil
}

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
