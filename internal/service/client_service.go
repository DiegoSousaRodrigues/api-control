package service

import (
	"github.com/autorei/api-control/internal/dto"
	"github.com/autorei/api-control/internal/repository"
	"strconv"
)

var ClientService IClientService = &clientService{}

type IClientService interface {
	List() (*[]dto.ClientDTO, error)
	Add(dto.ClientRequest) (err error)
	FindByID(id string) (*dto.ClientDTO, error)
	Update(id string, clientDto dto.ClientDTO) (err error)
}

type clientService struct{}

func (c *clientService) Update(id string, clientDto dto.ClientDTO) (err error) {
	entity, err := dto.ParseDtoToEntity(clientDto)
	if err != nil {
		return err
	}

	intId, err := strconv.Atoi(id)
	if err != nil {
		return err
	}

	err = repository.ClientRepository.Update(int64(intId), *entity)
	if err != nil {
		return nil
	}

	return nil
}

func (c *clientService) FindByID(id string) (*dto.ClientDTO, error) {
	entity, err := repository.ClientRepository.FindByID(id)
	if err != nil {
		return nil, err
	}

	dtoClient := dto.ParseToDTO(*entity)
	return &dtoClient, nil
}

func (c *clientService) Add(clientDTO dto.ClientRequest) (err error) {
	entity, err := dto.ParseRequestToEntity(clientDTO)
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
