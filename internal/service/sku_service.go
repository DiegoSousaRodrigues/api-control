package service

import (
	"github.com/autorei/api-control/internal/dto"
	"github.com/autorei/api-control/internal/repository"
)

var SkuService ISkuService = &skuService{}

type ISkuService interface {
	List() (*[]dto.SkuDTO, error)
	Add(clientDTO dto.SkuDTO) (err error)
}

type skuService struct{}

func (s *skuService) List() (*[]dto.SkuDTO, error) {
	listEntity, err := repository.SkuRepository.List()
	if err != nil {
		return nil, err
	}

	var listDTO []dto.SkuDTO

	for _, value := range *listEntity {
		listDTO = append(listDTO, dto.ParseSkuToDTO(value))
	}

	return &listDTO, nil
}

func (c *skuService) Add(clientDTO dto.SkuDTO) (err error) {
	entity, err := dto.ParseSkuRequestToEntity(clientDTO)
	if err != nil {
		return err
	}

	err = repository.SkuRepository.Add(*entity)
	if err != nil {
		return err
	}

	return nil
}
