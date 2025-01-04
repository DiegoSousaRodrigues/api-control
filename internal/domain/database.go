package domain

import (
	"log"
	"os"
	"sync"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // Usa o stdout para logar
		logger.Config{
			LogLevel:                  logger.Info,              // Loga todas as queries
			IgnoreRecordNotFoundError: true,                     // Ignorar erros de registro não encontrado
			Colorful:                  true,                     // Log colorido
		},
	)

	once.Do(func() {
		dsn := "postgres://neondb_owner:SOC3jPx8QpvA@ep-divine-mode-a46y0prw-pooler.us-east-1.aws.neon.tech/neondb?sslmode=require"

		var err error
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: newLogger,
		})
		if err != nil {
			log.Fatalf("Erro ao conectar com o banco de dados: %v", err)
		}
		log.Println("Conexão com o banco de dados realizada com sucesso!")
	})
	return db
}
