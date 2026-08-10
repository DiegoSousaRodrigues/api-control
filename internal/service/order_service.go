package service

import (
	"errors"
	"strconv"
	"time"
	_ "time/tzdata"

	"github.com/api-control/internal/dto"
	"github.com/api-control/internal/repository"
)

var OrderService IOrderService = &orderService{}

type IOrderService interface {
	List(year int16, month int16) (*[]dto.OrderResponseDTO, error)
	Add(orderDTO dto.OrderRequestDTO) error
	OpenBalance(clientID int64, year int16, month int16) (*dto.OpenBalanceDTO, error)
	FindByID(id string) (*dto.OrderResponseDTO, error)
	Update(id string, orderDTO dto.OrderRequestDTO) (err error)
}

type orderService struct{}

var (
	ErrOrderFutureCompetence = errors.New("future order competence is not allowed")
	orderNow                 = time.Now
)

func (s *orderService) List(year int16, month int16) (*[]dto.OrderResponseDTO, error) {
	if err := validateCompetence(year, month); err != nil {
		return nil, err
	}
	listEntity, err := repository.OrderRepository.List(year, month)
	if err != nil {
		return nil, err
	}

	var listDTO []dto.OrderResponseDTO

	for _, value := range *listEntity {
		listDTO = append(listDTO, dto.ParseOrderToDTO(value))
	}

	return &listDTO, nil
}

func (c *orderService) Add(orderDTO dto.OrderRequestDTO) error {
	entity, err := dto.ParseOrderRequestToEntity(orderDTO)
	if err != nil {
		return err
	}
	if err := validateCompetence(*entity.OrderYear, *entity.OrderMonth); err != nil {
		return err
	}

	err = repository.OrderRepository.Add(*entity)
	if err != nil {
		return err
	}
	return nil
}

func (c *orderService) OpenBalance(clientID int64, year int16, month int16) (*dto.OpenBalanceDTO, error) {
	if clientID <= 0 {
		return nil, dto.ErrOrderClientRequired
	}
	if err := validateCompetence(year, month); err != nil {
		return nil, err
	}
	balance, err := repository.OrderRepository.OpenBalance(clientID, year, month)
	if err != nil {
		return nil, err
	}
	return &dto.OpenBalanceDTO{ClientID: clientID, OrderYear: year, OrderMonth: month, Balance: dto.NewMoney(balance)}, nil
}

func validateCompetence(year int16, month int16) error {
	if year < 1 {
		return dto.ErrOrderYearInvalid
	}
	if month < 1 || month > 12 {
		return dto.ErrOrderMonthInvalid
	}
	location, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return err
	}
	now := orderNow().In(location)
	if int(year) > now.Year() || (int(year) == now.Year() && int(month) > int(now.Month())) {
		return ErrOrderFutureCompetence
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
	_, err = strconv.Atoi(id)
	if err != nil {
		return err
	}
	return repository.ErrOrderFinancialUpdateUnsupported
}
