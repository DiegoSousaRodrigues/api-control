package main

import (
	routes "github.com/autorei/api-control/internal"
)

func main() {
	//database.InitialMigration.InitialMigration()
	routes.Server.Run("3001")
}
