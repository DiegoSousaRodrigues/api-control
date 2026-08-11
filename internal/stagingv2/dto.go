package stagingv2

import "github.com/api-control/internal/dto"

type LoginRequest struct {
	Login    string `json:"login" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string      `json:"token"`
	User  UserSummary `json:"user"`
}

type UserSummary struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Login string `json:"login"`
}

type ClientRequest struct {
	Name             string  `json:"name" binding:"required"`
	Document         string  `json:"document" binding:"required"`
	BirthDate        *string `json:"birthDate"`
	Phone            string  `json:"phone" binding:"required"`
	SecondaryPhone   *string `json:"secondaryPhone"`
	Street           string  `json:"street" binding:"required"`
	Neighborhood     string  `json:"neighborhood" binding:"required"`
	AddressNumber    string  `json:"addressNumber" binding:"required"`
	Complement       *string `json:"complement"`
	PostalCode       *string `json:"postalCode"`
	AddressType      string  `json:"addressType" binding:"required"`
	AddressReference *string `json:"addressReference"`
	Position         int     `json:"position"`
}

type ClientResponse struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
	ClientRequest
}

type ProductRequest struct {
	Name          string     `json:"name" binding:"required"`
	PurchasePrice *dto.Money `json:"purchasePrice" binding:"required"`
	SalePrice     *dto.Money `json:"salePrice" binding:"required"`
	ImageURL      *string    `json:"imageUrl"`
}

type ProductResponse struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	PurchasePrice *dto.Money `json:"purchasePrice"`
	SalePrice     dto.Money  `json:"salePrice"`
	ImageURL      *string    `json:"imageUrl,omitempty"`
	Active        bool       `json:"active"`
}

type StatusRequest struct {
	Active *bool `json:"active" binding:"required"`
}
