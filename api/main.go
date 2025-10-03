package main

import (
	"log"

	routes "github.com/api-control/internal"
	"github.com/joho/godotenv"
	//database "github.com/api-control/internal/migrations"
)

func main() {
	//database.InitialMigration.InitialMigration()
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}
  	routes.Server.Run("3001")
}
