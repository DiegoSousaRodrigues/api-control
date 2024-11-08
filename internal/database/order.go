package database

import "time"

type Order struct {
	ID          int64     `json:"id"          gorm:"primaryKey;autoIncrement;not null"`
	DateCreated time.Time `json:"dateCreated" gorm:"not null;default:current_timestamp"`
	LastUpdated time.Time `json:"lastUpdated" gorm:"not null;default:current_timestamp"`
	ClientId    int64     `json:"clientId"    gorm:"not null;index"`
	Active      bool      `json:"active"      gorm:"not null;default:true"`

	Client    Client     `json:"client"       gorm:"foreignKey:ClientId;references:ID"`
	OrderSkus []OrderSku `json:"orderSkus"    gorm:"foreignKey:OrderID"` // Relacionamento has-many com OrderSku

}

// TableName retorna o nome da tabela para o GORM
func (Order) TableName() string {
	return "order" // Nome da tabela definido para o singular
}
