package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"
)

// SessionTTL is the fixed session lifetime; the API adds it to the clock's now.
const SessionTTL = 30 * 24 * time.Hour

// NewSessionToken returns the token to hand to the client and the hash to store.
func NewSessionToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

// apiTokenPrefix marks personal tokens, so a leaked string is recognizable.
const apiTokenPrefix = "ts_"

// NewAPIToken returns the personal token to show once and the hash to store.
func NewAPIToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token = apiTokenPrefix + base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

// HashToken is how a presented token is looked up: sessions and API tokens are stored hashed.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
