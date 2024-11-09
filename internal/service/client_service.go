package service

import (
	"github.com/autorei/api-control/internal/domain"
	"github.com/autorei/api-control/internal/repository"
)

var ClientService IClientService = &clientService{}

type IClientService interface {
	List() *[]domain.Client
}

type clientService struct {}

func (c *clientService) List() *[]domain.Client {
	list, err := repository.ClientRepository.List()
	if err != nil {
		return nil
	}

	//TODO fazer o parse para um DTO e fazer o return

	return list
}
