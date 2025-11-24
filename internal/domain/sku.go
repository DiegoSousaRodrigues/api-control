package domain

import "time"

type Sku struct {
	ID          int64      `json:"id"              gorm:"primaryKey;autoIncrement;not null"`
	DateCreated time.Time  `json:"dateCreated"     gorm:"not null;default:current_timestamp"`
	LastUpdated time.Time  `json:"lastUpdated"     gorm:"not null;default:current_timestamp"`
	Name        string     `json:"name"            gorm:"not null"`
	Price       float64    `json:"price"           gorm:"not null"`
	Active      bool       `json:"active"          gorm:"not null;default:true"`
	ImageUrl    *string    `json:"image_url"       gorm:""`
	OrderSkus   []OrderSku `json:"orderSkus"       gorm:"foreignKey:SkuID"` // Relacionamento has-many com OrderSku
}

// TableName retorna o nome da tabela para o GORM
func (Sku) TableName() string {
	return "sku" // Nome da tabela definido para o singular
}
