package store

import "time"

// User is a person who can log in; Email is stored lowercased.
type User struct {
	ID           uint   `gorm:"primarykey"`
	Email        string `gorm:"uniqueIndex;not null"`
	Name         string `gorm:"not null"`
	PasswordHash string `gorm:"not null"`
	Timezone     string `gorm:"not null;default:UTC"`
	IsAdmin      bool   `gorm:"not null;default:false"`
	Active       bool   `gorm:"not null;default:true"`
	// MustChangePassword confines the user to /me, /me/password and /auth/logout.
	MustChangePassword bool `gorm:"not null;default:false"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Session is a login session, stored as a hash of its opaque token.
type Session struct {
	ID        uint      `gorm:"primarykey"`
	UserID    uint      `gorm:"index;not null"`
	TokenHash string    `gorm:"uniqueIndex;not null"` // sha256 hex, never the raw token
	ExpiresAt time.Time `gorm:"not null"`
	CreatedAt time.Time
}

// Membership roles. A user has at most one membership per team.
const (
	RoleLead   = "lead"
	RoleMember = "member"
)

// Team is a named group that owns projects and has leads. Archived teams grant nothing.
type Team struct {
	ID         uint   `gorm:"primarykey"`
	Name       string `gorm:"not null"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	ArchivedAt *time.Time
}

// TeamMembership links a user to a team with the role lead or member.
type TeamMembership struct {
	ID        uint   `gorm:"primarykey"`
	TeamID    uint   `gorm:"not null;uniqueIndex:idx_team_user"`
	UserID    uint   `gorm:"not null;uniqueIndex:idx_team_user"`
	Role      string `gorm:"not null;default:member"`
	CreatedAt time.Time
}

// Project is the unit of work entries are logged against. It belongs to one team.
type Project struct {
	ID        uint   `gorm:"primarykey"`
	TeamID    uint   `gorm:"not null;index"`
	Name      string `gorm:"not null"`
	Color     string
	CreatedAt time.Time
	UpdatedAt time.Time
	// Archived projects leave the pickers but keep their hours.
	ArchivedAt *time.Time
}

// APIToken is a personal token that acts as its user; only the hash is stored.
type APIToken struct {
	ID         uint   `gorm:"primarykey"`
	UserID     uint   `gorm:"not null;index"`
	Name       string `gorm:"not null"`
	TokenHash  string `gorm:"uniqueIndex;not null"` // sha256 hex, never the raw token
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}
