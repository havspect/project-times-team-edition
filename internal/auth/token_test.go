package auth

import (
	"encoding/base64"
	"regexp"
	"testing"
	"time"
)

func TestNewSessionToken(t *testing.T) {
	token, hash, err := NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	other, otherHash, err := NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if token == other || hash == otherHash {
		t.Error("two session tokens are identical")
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(hash) {
		t.Errorf("hash %q is not 64 lowercase hex chars", hash)
	}
	if HashToken(token) != hash {
		t.Error("HashToken(token) does not equal the returned hash")
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("token is not URL-safe base64: %v", err)
	}
	if len(raw) != 32 {
		t.Errorf("token decodes to %d bytes, want 32", len(raw))
	}
}

func TestSessionTTL(t *testing.T) {
	if SessionTTL != 30*24*time.Hour {
		t.Errorf("SessionTTL = %v, want 720h", SessionTTL)
	}
}
