package models

import (
	"time"
)

type Payment struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"index;not null" json:"user_id"`
	Amount      float64   `gorm:"not null" json:"amount"`
	Description string    `gorm:"size:100" json:"description"`
	CreatedAt   time.Time `json:"created_at"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}
