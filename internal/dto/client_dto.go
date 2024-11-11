package dto

import "github.com/autorei/api-control/internal/domain"

type (
	ClientDTO struct {
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
)

func ParseToDTO(entity domain.Client) ClientDTO {
	return ClientDTO{
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
