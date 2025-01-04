package main

import (
	routes "github.com/autorei/api-control/internal"
	//database "github.com/autorei/api-control/internal/migrations"
)

func main() {
	//database.InitialMigration.InitialMigration()
	routes.Server.Run("3001")
}
