package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Name         string         `gorm:"size:60;not null" json:"name"`
	Email        string         `gorm:"size:60;uniqueIndex;not null" json:"email"`
	PasswordHash string         `gorm:"size:64;not null" json:"-"`
	Role         Role           `gorm:"size:20;default:user" json:"role"`
	Plan         int            `gorm:"default:1" json:"plan"` // GB cinsinden kota
	IsApproved   bool           `gorm:"default:false" json:"is_approved"`
	IsActive     bool           `gorm:"default:true" json:"is_active"`
	ApprovedAt   *time.Time     `json:"approved_at,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	Devices  []Device  `gorm:"foreignKey:UserID" json:"devices,omitempty"`
	Payments []Payment `gorm:"foreignKey:UserID" json:"payments,omitempty"`
}

func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

func (u *User) Approve() {
	now := time.Now()
	u.IsApproved = true
	u.ApprovedAt = &now
}
