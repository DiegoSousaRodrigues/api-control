package main

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
)

func main() {
	// Defina a string de conexão para o PostgreSQL
	// A string de conexão tem o seguinte formato: "user=USER password=PASSWORD dbname=DBNAME host=HOST port=PORT sslmode=disable"
	dsn := "user=postgres dbname=postgres host=localhost port=5432 sslmode=disable"

	// Abrir a conexão com o PostgreSQL
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Erro ao conectar com o banco de dados: %v", err)
	}

	// Realizar a migração automática
	err = db.AutoMigrate(&User{})
	if err != nil {
		log.Fatalf("Erro ao executar a migração: %v", err)
	}

	log.Println("Migração realizada com sucesso!")
}
