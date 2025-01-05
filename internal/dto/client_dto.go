package dto

import (
	"strconv"

	"github.com/api-control/internal/domain"
)

type (
	ClientDTO struct {
		ID               int64   `json:"id"`
		Name             string  `json:"name"`
		Document         string  `json:"document"`
		Phone            string  `json:"phone"`
		Telephone        *string `json:"telephone"`
		Birthdate        string  `json:"birthdate"`
		Active           bool    `json:"active"`
		Street           string  `json:"street"`
		Quarter          string  `json:"quarter"`
		Number           string  `json:"number"`
		Complement       *string `json:"complement"`
		Zipcode          *string `json:"zipcode"`
		AddressType      string  `json:"addressType"`
		AddressReference *string `json:"addressReference"`
		Position         int     `json:"position"`
	}

	ClientRequest struct {
		ClientDTO
		Position string `json:"position"`
	}
)

func ParseToDTO(entity domain.Client) ClientDTO {
	return ClientDTO{
		ID:               entity.ID,
		Name:             entity.Name,
		Document:         entity.Document,
		Phone:            entity.Phone,
		Telephone:        entity.Telephone,
		Birthdate:        entity.Birthdate,
		Active:           entity.Active,
		Street:           entity.Street,
		Quarter:          entity.Quarter,
		Number:           entity.Number,
		Complement:       entity.Complement,
		Zipcode:          entity.Zipcode,
		AddressType:      entity.AddressType,
		AddressReference: entity.AddressReference,
		Position:         entity.Position,
	}
}

func ParseRequestToEntity(dto ClientRequest) (*domain.Client, error) {
	position, err := strconv.Atoi(dto.Position)
	if err != nil {
		return nil, err
	}

	return &domain.Client{
		Name:             dto.Name,
		Document:         dto.Document,
		Phone:            dto.Phone,
		Telephone:        dto.Telephone,
		Birthdate:        dto.Birthdate,
		Active:           true,
		Street:           dto.Street,
		Quarter:          dto.Quarter,
		Number:           dto.Number,
		Complement:       dto.Complement,
		Zipcode:          dto.Zipcode,
		AddressType:      dto.AddressType,
		AddressReference: dto.AddressReference,
		Position:         position,
	}, nil
}

func ParseDtoToEntity(dto ClientDTO) (*domain.Client, error) {
	return &domain.Client{
		Name:             dto.Name,
		Document:         dto.Document,
		Phone:            dto.Phone,
		Telephone:        dto.Telephone,
		Birthdate:        dto.Birthdate,
		Active:           true,
		Street:           dto.Street,
		Quarter:          dto.Quarter,
		Number:           dto.Number,
		Complement:       dto.Complement,
		Zipcode:          dto.Zipcode,
		AddressType:      dto.AddressType,
		AddressReference: dto.AddressReference,
		Position:         dto.Position,
	}, nil
}
