package store

import (
	"errors"

	"gorm.io/gorm"
)

// ErrLastAdmin is returned when an update would leave the workspace without an active admin.
var ErrLastAdmin = errors.New("last admin")

// ListUsers returns every user, deactivated ones included, ordered by email.
func (s *Store) ListUsers() ([]User, error) {
	var users []User
	err := s.db.Order("email").Find(&users).Error
	return users, err
}

// SaveUser writes the admin-editable fields of u and refuses to leave no active admin.
func (s *Store) SaveUser(u *User) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var current User
		if err := tx.First(&current, u.ID).Error; err != nil {
			return translate(err)
		}
		if current.IsAdmin && current.Active && (!u.IsAdmin || !u.Active) {
			// Count and write in one transaction so two concurrent demotions cannot both pass.
			var admins int64
			err := tx.Model(&User{}).Where("is_admin AND active").Count(&admins).Error
			if err != nil {
				return err
			}
			if admins <= 1 {
				return ErrLastAdmin
			}
		}
		return tx.Model(&User{}).Where("id = ?", u.ID).Updates(map[string]any{
			"name":     u.Name,
			"timezone": u.Timezone,
			"is_admin": u.IsAdmin,
			"active":   u.Active,
		}).Error
	})
}

// SetUserPassword stores hash and sets must_change_password; an unknown id returns ErrNotFound.
func (s *Store) SetUserPassword(id uint, hash string, mustChange bool) error {
	res := s.db.Model(&User{}).Where("id = ?", id).
		Updates(map[string]any{"password_hash": hash, "must_change_password": mustChange})
	if res.Error == nil && res.RowsAffected == 0 {
		return ErrNotFound
	}
	return res.Error
}
