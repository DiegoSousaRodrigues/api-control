package main

import (
	"log"

	routes "github.com/api-control/internal"
	"github.com/api-control/internal/utils"
	// database "github.com/api-control/internal/migrations"
	"github.com/joho/godotenv"
)

func main() {
	// database.InitialMigration.InitialMigration()
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	if err := utils.ValidateJWTConfiguration(); err != nil {
		log.Fatalf("invalid JWT configuration: %v", err)
	}

	routes.Server.Run("3001")
}
