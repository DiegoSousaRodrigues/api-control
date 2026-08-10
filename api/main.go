package main

import (
	"context"
	"log"
	"time"

	routes "github.com/api-control/internal"
	database "github.com/api-control/internal/migrations"
	"github.com/api-control/internal/utils"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	if err := utils.ValidateJWTConfiguration(); err != nil {
		log.Fatalf("invalid JWT configuration: %v", err)
	}

	migrationContext, cancelMigrations := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancelMigrations()
	if err := database.Migrations.Run(migrationContext); err != nil {
		log.Fatalf("failed to apply database migrations: %v", err)
	}

	routes.Server.Run("3001")
}
