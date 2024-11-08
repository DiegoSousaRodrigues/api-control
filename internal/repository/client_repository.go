package repository

import (
	"fmt"
	"github.com/autorei/api-control/internal/database"
	database2 "github.com/autorei/api-control/internal/migrations"
	"log"
)

func List() (*[]database.Client) {
	entity := &[]database.Client{}
	db := database2.InitDB()

	if err := db.Find(entity); err.Error != nil {
		log.Fatalf("Erro ao buscar clientes: %v", err)
	}

	for _, value := range *entity{
		fmt.Println(value)
	}

	return entity
}