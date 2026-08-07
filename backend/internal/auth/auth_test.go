package auth

import (
	"testing"
	"time"
)

const testSecret = "unit-test-secret"

func TestIssueParseRoundtrip(t *testing.T) {
	tok, err := IssueToken(testSecret, time.Hour, "user-1", "admin", "admin")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	claims, err := ParseToken(testSecret, tok)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-1" || claims.Username != "admin" || claims.RoleCode != "admin" {
		t.Errorf("claims = %+v", claims)
	}
	if claims.ExpiresAt == nil {
		t.Fatal("missing expiry")
	}
}

func TestParseTokenTampered(t *testing.T) {
	tok, _ := IssueToken(testSecret, time.Hour, "user-1", "admin", "admin")
	tampered := tok[:len(tok)-3] + "abc"
	if _, err := ParseToken(testSecret, tampered); err == nil {
		t.Fatal("tampered token accepted")
	}
}

func TestParseTokenWrongSecret(t *testing.T) {
	tok, _ := IssueToken(testSecret, time.Hour, "user-1", "admin", "admin")
	if _, err := ParseToken("other-secret", tok); err == nil {
		t.Fatal("token accepted with wrong secret")
	}
}

func TestParseTokenExpired(t *testing.T) {
	tok, err := IssueToken(testSecret, -time.Minute, "user-1", "admin", "admin")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if _, err := ParseToken(testSecret, tok); err == nil {
		t.Fatal("expired token accepted")
	}
}

func TestPasswordHashAndCheck(t *testing.T) {
	hash, err := HashPassword("s3cret!")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "s3cret!" {
		t.Fatal("hash must not equal plaintext")
	}
	if !CheckPassword(hash, "s3cret!") {
		t.Fatal("correct password rejected")
	}
	if CheckPassword(hash, "wrong") {
		t.Fatal("wrong password accepted")
	}
}

func TestSessionHashStable(t *testing.T) {
	a := SessionHash("token", "user-1")
	b := SessionHash("token", "user-1")
	if a != b {
		t.Fatal("session hash not deterministic")
	}
	if a == SessionHash("token", "user-2") {
		t.Fatal("session hash should differ per user")
	}
	if a == SessionHash("other", "user-1") {
		t.Fatal("session hash should differ per token")
	}
	if len(a) != 32 {
		t.Errorf("session hash length = %d, want 32 (md5 hex)", len(a))
	}
}
