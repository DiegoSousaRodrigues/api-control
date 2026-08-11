package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/api-control/internal/admin"
	"github.com/api-control/internal/stagingv2"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	_ = godotenv.Load()
	err := admin.Run(context.Background(), os.Args[1:], os.Stderr, readPasswordSecurely, openInitialUserStore)
	if err != nil {
		fmt.Fprintln(os.Stderr, "control-admin operation failed")
		os.Exit(1)
	}
}

func openInitialUserStore(context.Context) (admin.InitialUserStore, func() error, error) {
	dsn := os.Getenv("DB_CONNECTION_STRING")
	if dsn == "" {
		return nil, nil, errors.New("database configuration is missing")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, nil, errors.New("cannot connect to staging database")
	}
	repository, err := stagingv2.NewRepository(db)
	if err != nil {
		return nil, nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, errors.New("cannot access staging database connection")
	}
	return repository, sqlDB.Close, nil
}
