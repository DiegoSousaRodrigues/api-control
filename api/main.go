package main

import (
	"log"

	routes "github.com/api-control/internal"
	// database "github.com/api-control/internal/migrations"
	"github.com/joho/godotenv"
)

func main() {
	// database.InitialMigration.InitialMigration()
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}
	routes.Server.Run("3001")
}
