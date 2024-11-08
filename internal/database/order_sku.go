package database

import "time"

type OrderSku struct {
	ID          int64     `json:"id"          gorm:"primaryKey;autoIncrement;not null"`
	DateCreated time.Time `json:"dateCreated" gorm:"not null;default:current_timestamp"`
	LastUpdated time.Time `json:"lastUpdated" gorm:"not null;default:current_timestamp"`
	Name        string    `json:"name"        gorm:"not null"`
	Price       float64   `json:"price"       gorm:"not null"`
	Quantity    int       `json:"quantity"    gorm:"not null"`

	// Relacionamento com Order
	OrderID int64  `json:"orderId" gorm:"not null;index"` // Chave estrangeira para Order
	Order   Order  `json:"order"   gorm:"foreignKey:OrderID;references:ID"`

	// Relacionamento com Sku
	SkuID int64 `json:"skuId" gorm:"not null;index"` // Chave estrangeira para Sku
	Sku   Sku   `json:"sku"   gorm:"foreignKey:SkuID;references:ID"`
}

// TableName retorna o nome da tabela para o GORM
func (OrderSku) TableName() string {
	return "order_sku" // Nome da tabela definido para o singular
}

