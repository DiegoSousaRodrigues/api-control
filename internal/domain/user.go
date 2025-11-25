package domain

import "time"

type User struct {
	ID          int64     `json:"id"          gorm:"primaryKey;autoIncrement;not null"`
	DateCreated time.Time `json:"dateCreated" gorm:"not null;default:current_timestamp"`
	LastUpdated time.Time `json:"lastUpdated" gorm:"not null;default:current_timestamp"`
	Name        string    `json:"name"        gorm:"not null"`
	Login       string    `json:"login"       gorm:"uniqueIndex;not null"`
	Password    string    `json:"-"           gorm:"not null"` // Não expor na resposta JSON
	Active      bool      `json:"active"      gorm:"not null;default:true"`
}

// TableName retorna o nome da tabela para o GORM
func (User) TableName() string {
	return "user" // Nome da tabela definido para o singular
}
