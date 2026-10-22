// Package store persists users and sessions in SQLite through GORM.
package store

import (
	"errors"
	"os"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SchemaVersion is reported by /healthz so support can tell which model set a database has.
const SchemaVersion = 1

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("not found")

// Store is a handle to the SQLite database.
type Store struct {
	db *gorm.DB
}

// Open opens the SQLite database at path, creating the file 0600 when it does not exist.
// File databases get WAL and a 10s busy timeout, so a second writer waits instead of
// failing with "database is locked" immediately.
func Open(path string) (*Store, error) {
	dsn := path
	// A DSN with options is not a plain file, and :memory: has none to protect.
	if path != ":memory:" && !strings.Contains(path, "?") {
		// The file holds password and session hashes, so a new one must not be world-readable.
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, err
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
		dsn = path + "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)"
	}
	// The default logger prints every not-found lookup, which every bad login and
	// stale cookie triggers; a missing row is a normal reply here, not an error.
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close closes the underlying SQL connection pool.
func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Migrate creates or updates the tables; running it on every boot is safe.
func (s *Store) Migrate() error {
	return s.db.AutoMigrate(&User{}, &Session{})
}

// CreateUser inserts u, lowercasing and trimming u.Email in place.
func (s *Store) CreateUser(u *User) error {
	u.Email = normalizeEmail(u.Email)
	return s.db.Create(u).Error
}

// UserByID returns the user with id, or ErrNotFound.
func (s *Store) UserByID(id uint) (*User, error) {
	var u User
	if err := s.db.First(&u, id).Error; err != nil {
		return nil, translate(err)
	}
	return &u, nil
}

// UserByEmail matches email case-insensitively and returns ErrNotFound when nobody has it.
func (s *Store) UserByEmail(email string) (*User, error) {
	var u User
	if err := s.db.Where("email = ?", normalizeEmail(email)).First(&u).Error; err != nil {
		return nil, translate(err)
	}
	return &u, nil
}

// CreateSession stores a session; TokenHash must already be hashed by auth.HashToken.
func (s *Store) CreateSession(sess *Session) error {
	return s.db.Create(sess).Error
}

// SessionByTokenHash looks up a session by auth.HashToken(token).
// It does not check ExpiresAt; callers do.
func (s *Store) SessionByTokenHash(hash string) (*Session, error) {
	var sess Session
	if err := s.db.Where("token_hash = ?", hash).First(&sess).Error; err != nil {
		return nil, translate(err)
	}
	return &sess, nil
}

// DeleteSession removes a session; deleting one that is already gone is not an error.
func (s *Store) DeleteSession(id uint) error {
	return s.db.Delete(&Session{}, id).Error
}

// SetUserActive enables or disables a user's ability to log in and use tokens.
// An unknown id returns ErrNotFound.
func (s *Store) SetUserActive(id uint, active bool) error {
	res := s.db.Model(&User{}).Where("id = ?", id).Update("active", active)
	if res.Error == nil && res.RowsAffected == 0 {
		return ErrNotFound
	}
	return res.Error
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func translate(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
