package repository

import (
	"github.com/ilker/backup-server/internal/config"
	"github.com/ilker/backup-server/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewDatabase(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(cfg.SQLitePath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	// Auto migrate
	err = db.AutoMigrate(
		&models.User{},
		&models.Device{},
		&models.Backup{},
		&models.Catalog{},
		&models.Payment{},
	)
	if err != nil {
		return nil, err
	}

	return db, nil
}
