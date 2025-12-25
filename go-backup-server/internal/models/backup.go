package models

import (
	"time"

	"gorm.io/gorm"
)

type Backup struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	DeviceID  uint           `gorm:"index;not null" json:"device_id"`
	FileName  string         `gorm:"size:255;not null" json:"file_name"`
	FilePath  string         `gorm:"size:500;not null" json:"-"`
	FileSize  int64          `gorm:"not null" json:"file_size"`
	Checksum  string         `gorm:"size:64" json:"checksum"` // SHA256
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Device Device `gorm:"foreignKey:DeviceID" json:"-"`
}

func (b *Backup) FileSizeMB() float64 {
	return float64(b.FileSize) / (1024 * 1024)
}

// Catalog represents an encrypted catalog file (SQLite dump)
type Catalog struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	DeviceID  uint           `gorm:"index;not null" json:"device_id"`
	SessionID string         `gorm:"size:50;not null" json:"session_id"` // YYYYMMDD-HHMMSS format
	FileName  string         `gorm:"size:255;not null" json:"file_name"`
	FilePath  string         `gorm:"size:500;not null" json:"-"`
	FileSize  int64          `gorm:"not null" json:"file_size"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Device Device `gorm:"foreignKey:DeviceID" json:"-"`
}
