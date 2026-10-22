package store

import (
	"errors"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SchemaVersion is reported by /healthz so support can tell which model set a database has.
const SchemaVersion = 1

var ErrNotFound = errors.New("not found")

type Store struct {
	db *gorm.DB
}

func Open(path string) (*Store, error) {
	// The default logger prints every not-found lookup, which every bad login and
	// stale cookie triggers; a missing row is a normal reply here, not an error.
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (s *Store) Migrate() error {
	return s.db.AutoMigrate(&User{}, &Session{})
}

func (s *Store) DB() *gorm.DB {
	return s.db
}

func (s *Store) CreateUser(u *User) error {
	u.Email = normalizeEmail(u.Email)
	return s.db.Create(u).Error
}

func (s *Store) UserByID(id uint) (*User, error) {
	var u User
	if err := s.db.First(&u, id).Error; err != nil {
		return nil, translate(err)
	}
	return &u, nil
}

func (s *Store) UserByEmail(email string) (*User, error) {
	var u User
	if err := s.db.Where("email = ?", normalizeEmail(email)).First(&u).Error; err != nil {
		return nil, translate(err)
	}
	return &u, nil
}

func (s *Store) CreateSession(sess *Session) error {
	return s.db.Create(sess).Error
}

func (s *Store) SessionByTokenHash(hash string) (*Session, error) {
	var sess Session
	if err := s.db.Where("token_hash = ?", hash).First(&sess).Error; err != nil {
		return nil, translate(err)
	}
	return &sess, nil
}

func (s *Store) DeleteSession(id uint) error {
	return s.db.Delete(&Session{}, id).Error
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
