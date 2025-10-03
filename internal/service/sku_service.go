package service

import (
	"strconv"

	"github.com/api-control/internal/dto"
	"github.com/api-control/internal/repository"
)

var SkuService ISkuService = &skuService{}

type ISkuService interface {
	List() (*[]dto.SkuDTO, error)
	Add(clientDTO dto.SkuDTO) (err error)
	ChangeStatus(id string, status string) error
	FindByID(id string) (*dto.SkuDTO, error)
	Update(id string, skuDto dto.SkuDTO) (err error)
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

func (c *skuService) ChangeStatus(id string, status string) error {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return err
	}

	statusBool, err := strconv.ParseBool(status)
	if err != nil {
		return err
	}

	err = repository.SkuRepository.ChangeStatus(int64(idInt), statusBool)
	if err != nil {
		return err
	}

	return nil
}

func (c *skuService) FindByID(id string) (*dto.SkuDTO, error) {
	entity, err := repository.SkuRepository.FindByID(id)
	if err != nil {
		return nil, err
	}

	dtoSku := dto.ParseSkuToDTO(*entity)
	return &dtoSku, nil
}

func (c *skuService) Update(id string, skuDto dto.SkuDTO) (err error) {
	entity, err := dto.ParseSkuRequestToEntity(skuDto)
	if err != nil {
		return err
	}

	intId, err := strconv.Atoi(id)
	if err != nil {
		return err
	}

	err = repository.SkuRepository.Update(int64(intId), *entity)
	if err != nil {
		return nil
	}

	return nil
}