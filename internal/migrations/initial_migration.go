package database

import (
	"fmt"
	"github.com/autorei/api-control/internal/database"
)

// InitialMigration realiza a migração inicial
func InitialMigration() {
	db := InitDB()

	// Realiza a migração
	err := db.AutoMigrate(&database.Client{})
	if err != nil {
		fmt.Printf("Erro ao executar a migração Client: %v\n", err)
		return
	}

	err = db.AutoMigrate(&database.Sku{})
	if err != nil {
		fmt.Printf("Erro ao executar a migração Sku: %v\n", err)
		return
	}

	err = db.AutoMigrate(&database.Order{})
	if err != nil {
		fmt.Printf("Erro ao executar a migração Order: %v\n", err)
		return
	}

	err = db.AutoMigrate(&database.OrderSku{})
	if err != nil {
		fmt.Printf("Erro ao executar a migração OrderSku: %v\n", err)
		return
	}

	err = db.AutoMigrate(&database.Client{})
	if err != nil {
		fmt.Printf("Erro ao executar a migração: %v\n", err)
		return
	}

	fmt.Println("Migração realizada com sucesso!")
}
