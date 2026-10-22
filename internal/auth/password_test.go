package auth

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestHashPasswordRoundTrip(t *testing.T) {
	encoded, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=65536,t=1,p=4$") {
		t.Fatalf("unexpected PHC encoding: %q", encoded)
	}
	if !VerifyPassword(encoded, "correct horse battery staple") {
		t.Error("VerifyPassword rejected the right password")
	}
	if VerifyPassword(encoded, "wrong password") {
		t.Error("VerifyPassword accepted the wrong password")
	}
	if VerifyPassword(encoded, "") {
		t.Error("VerifyPassword accepted an empty password")
	}
}

func TestHashPasswordUsesFreshSalt(t *testing.T) {
	a, err := HashPassword("same")
	if err != nil {
		t.Fatal(err)
	}
	b, err := HashPassword("same")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Error("two hashes of the same password are identical; salt is not random")
	}
	if !VerifyPassword(a, "same") || !VerifyPassword(b, "same") {
		t.Error("at least one hash does not verify its password")
	}
}

func TestVerifyPasswordRejectsMalformed(t *testing.T) {
	good, err := HashPassword("secret")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(good, "$")

	cases := map[string]string{
		"empty":             "",
		"garbage":           "not-a-hash",
		"plaintext":         "secret",
		"too few fields":    "$argon2id$v=19$m=65536,t=1,p=4$" + parts[4],
		"too many fields":   good + "$extra",
		"unknown algorithm": "$argon2i$v=19$m=65536,t=1,p=4$" + parts[4] + "$" + parts[5],
		"wrong version":     "$argon2id$v=18$m=65536,t=1,p=4$" + parts[4] + "$" + parts[5],
		"truncated salt":    "$argon2id$v=19$m=65536,t=1,p=4$" + parts[4][:3] + "$" + parts[5],
		"truncated hash":    "$argon2id$v=19$m=65536,t=1,p=4$" + parts[4] + "$" + parts[5][:5],
		"empty hash":        "$argon2id$v=19$m=65536,t=1,p=4$" + parts[4] + "$",
		"bad params":        "$argon2id$v=19$m=x,t=1,p=4$" + parts[4] + "$" + parts[5],
		"missing params":    "$argon2id$v=19$$" + parts[4] + "$" + parts[5],
		"zero threads":      "$argon2id$v=19$m=65536,t=1,p=0$" + parts[4] + "$" + parts[5],
		"huge memory":       "$argon2id$v=19$m=4294967295,t=1,p=4$" + parts[4] + "$" + parts[5],
	}
	for name, encoded := range cases {
		t.Run(name, func(t *testing.T) {
			if VerifyPassword(encoded, "secret") {
				t.Errorf("VerifyPassword accepted %q", encoded)
			}
		})
	}
}

func TestGeneratePassword(t *testing.T) {
	a, err := GeneratePassword()
	if err != nil {
		t.Fatal(err)
	}
	b, err := GeneratePassword()
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 20 {
		t.Errorf("length = %d, want 20", len(a))
	}
	if a == b {
		t.Error("two generated passwords are identical")
	}
	raw, err := base64.RawURLEncoding.DecodeString(a)
	if err != nil {
		t.Fatalf("not URL-safe base64: %v", err)
	}
	if len(raw) != 15 {
		t.Errorf("decoded %d bytes, want 15", len(raw))
	}
	for _, r := range a {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_'
		if !ok {
			t.Errorf("character %q is not URL-safe", r)
		}
	}
}
