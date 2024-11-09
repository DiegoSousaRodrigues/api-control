package domain

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"sync"
)

type BaseRepository struct {}

var (
	db   *gorm.DB
	once sync.Once
)

func (b BaseRepository) PSQL() *gorm.DB {
	return b.OpenConn()
}

func (b BaseRepository) OpenConn() *gorm.DB {
	once.Do(func() {
		dsn := "user=postgres dbname=postgres host=localhost port=5432 sslmode=disable"

		var err error
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			log.Fatalf("Erro ao conectar com o banco de dados: %v", err)
		}
		log.Println("Conexão com o banco de dados realizada com sucesso!")
	})
	return db
}
