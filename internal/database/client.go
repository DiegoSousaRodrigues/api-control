package database

import "time"

type Client struct {
	ID               int64     `json:"id"              gorm:"primaryKey;autoIncrement;not null"`
	DateCreated      time.Time `json:"dateCreated"     gorm:"not null;default:current_timestamp"`
	LastUpdated      time.Time `json:"lastUpdated"     gorm:"not null;default:current_timestamp"`
	Name             string    `json:"name"            gorm:"not null"`
	Document         string    `json:"document"        gorm:"not null"`
	Phone            string    `json:"phone"           gorm:"not null"`
	Telephone        *string   `json:"telephone"       gorm:""`
	Birthdate        string    `json:"birthdate"       gorm:"not null"`
	Active           bool      `json:"active"          gorm:"not null;default:true"`
	Street           string    `json:"street"          gorm:"not null"`
	Quarter          string    `json:"quarter"         gorm:"not null"`
	Number           string    `json:"number"          gorm:"not null"`
	Complement       *string   `json:"complement"      gorm:""`
	Zipcode          *string   `json:"zipcode"         gorm:""`
	AddressType      string    `json:"addressType"     gorm:"not null"`
	AddressReference *string   `json:"addressReference" gorm:""`
	Position         int       `json:"position"        gorm:"autoIncrement;not null"`

	Orders []Order `json:"orders" gorm:"foreignKey:ClientId"` // Relacionamento has-many
}

// TableName retorna o nome da tabela para o GORM
func (Client) TableName() string {
	return "client" // Nome da tabela definido para o singular
}

