package service

import (
	"strconv"

	"github.com/api-control/internal/dto"
	"github.com/api-control/internal/repository"
)

var OrderService IOrderService = &orderService{}

type IOrderService interface {
	List() (*[]dto.OrderResponseDTO, error)
	Add(orderDTO dto.OrderRequestDTO) (error)
	ChangeStatus(id string, status string) error
	FindByID(id string) (*dto.OrderResponseDTO, error)
	Update(id string, orderDTO dto.OrderRequestDTO) (err error)
}

type orderService struct{}

func (s *orderService) List() (*[]dto.OrderResponseDTO, error) {
	listEntity, err := repository.OrderRepository.List()
	if err != nil {
		return nil, err
	}

	var listDTO []dto.OrderResponseDTO

	for _, value := range *listEntity {
		listDTO = append(listDTO, dto.ParseOrderToDTO(value))
	}

	return &listDTO, nil
}

func (c *orderService) Add(orderDTO dto.OrderRequestDTO) (error) {
	entity, err := dto.ParseOrderRequestToEntity(orderDTO)
	if err != nil {
		return err
	}

	err = repository.OrderRepository.Add(*entity)
	if err != nil {
		return err
	}
	return nil
}

func (c *orderService) ChangeStatus(id string, status string) error {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return err
	}

	statusBool, err := strconv.ParseBool(status)
	if err != nil {
		return err
	}

	err = repository.OrderRepository.ChangeStatus(int64(idInt), statusBool)
	if err != nil {
		return err
	}

	return nil
}

func (c *orderService) FindByID(id string) (*dto.OrderResponseDTO, error) {
	entity, err := repository.OrderRepository.FindByID(id)
	if err != nil {
		return nil, err
	}

	dtoOrder := dto.ParseOrderToDTO(*entity)
	return &dtoOrder, nil
}

func (c *orderService) Update(id string, orderDTO dto.OrderRequestDTO) (err error) {
	entity, err := dto.ParseOrderRequestToEntity(orderDTO)
	if err != nil {
		return nil
	}

	intId, err := strconv.Atoi(id)
	if err != nil {
		return err
	}

	err = repository.OrderRepository.Update(int64(intId), *entity)
	if err != nil {
		return nil
	}

	return nil
}