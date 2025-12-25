package models

import (
	"time"

	"gorm.io/gorm"
)

type Device struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"size:100;not null" json:"name"`
	UserID    uint           `gorm:"index;not null" json:"user_id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	User    User     `gorm:"foreignKey:UserID" json:"-"`
	Backups []Backup `gorm:"foreignKey:DeviceID" json:"backups,omitempty"`
}
