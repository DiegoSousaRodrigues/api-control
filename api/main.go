package main

import (
	database "github.com/autorei/api-control/internal/migrations"
	"github.com/autorei/api-control/internal/repository"
)

func main() {
	database.InitialMigration()
	repository.List()
}
