package database

import (
	"fmt"
	"github.com/api-control/internal/domain"
)

var InitialMigration IInitialMigration = &initialMigration{}

type IInitialMigration interface {
	InitialMigration()
}

type initialMigration struct {
	db domain.BaseRepository
}

// InitialMigration realiza a migração inicial
func (im *initialMigration) InitialMigration() {
	db := im.db.PSQL()

	// Realiza a migração
	err := db.AutoMigrate(&domain.Client{})
	if err != nil {
		fmt.Printf("Erro ao executar a migração Client: %v\n", err)
		return
	}

	err = db.AutoMigrate(&domain.Sku{})
	if err != nil {
		fmt.Printf("Erro ao executar a migração Sku: %v\n", err)
		return
	}

	err = db.AutoMigrate(&domain.Order{})
	if err != nil {
		fmt.Printf("Erro ao executar a migração Order: %v\n", err)
		return
	}

	err = db.AutoMigrate(&domain.OrderSku{})
	if err != nil {
		fmt.Printf("Erro ao executar a migração OrderSku: %v\n", err)
		return
	}

	err = db.AutoMigrate(&domain.Client{})
	if err != nil {
		fmt.Printf("Erro ao executar a migração: %v\n", err)
		return
	}

	fmt.Println("Migração realizada com sucesso!")
}
