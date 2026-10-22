package store

import "time"

// CreateAPIToken stores a personal token; TokenHash must already be auth.HashToken(raw).
func (s *Store) CreateAPIToken(t *APIToken) error {
	return s.db.Create(t).Error
}

// RevokeAPIToken marks a token dead; an unknown id returns ErrNotFound.
func (s *Store) RevokeAPIToken(id uint, at time.Time) error {
	res := s.db.Model(&APIToken{}).Where("id = ? AND revoked_at IS NULL", id).Update("revoked_at", at)
	if res.Error == nil && res.RowsAffected == 0 {
		return ErrNotFound
	}
	return res.Error
}

// APITokenByTokenHash looks up a personal token by auth.HashToken(value).// It does not check RevokedAt; callers do.
func (s *Store) APITokenByTokenHash(hash string) (*APIToken, error) {
	var t APIToken
	if err := s.db.Where("token_hash = ?", hash).First(&t).Error; err != nil {
		return nil, translate(err)
	}
	return &t, nil
}

// TouchAPIToken records when the token was last used.
func (s *Store) TouchAPIToken(id uint, at time.Time) error {
	return s.db.Model(&APIToken{}).Where("id = ?", id).Update("last_used_at", at).Error
}
