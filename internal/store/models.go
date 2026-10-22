package store

import "time"

type User struct {
	ID           uint   `gorm:"primarykey"`
	Email        string `gorm:"uniqueIndex;not null"`
	Name         string `gorm:"not null"`
	PasswordHash string `gorm:"not null"`
	Timezone     string `gorm:"not null;default:UTC"`
	IsAdmin      bool   `gorm:"not null;default:false"`
	Active       bool   `gorm:"not null;default:true"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Session struct {
	ID        uint      `gorm:"primarykey"`
	UserID    uint      `gorm:"index;not null"`
	TokenHash string    `gorm:"uniqueIndex;not null"` // sha256 hex, never the raw token
	ExpiresAt time.Time `gorm:"not null"`
	CreatedAt time.Time
}
