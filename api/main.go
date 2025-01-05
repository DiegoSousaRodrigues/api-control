package main

import (
	routes "github.com/api-control/internal"
	//database "github.com/api-control/internal/migrations"
)

func main() {
	//database.InitialMigration.InitialMigration()
	routes.Server.Run("3001")
}
