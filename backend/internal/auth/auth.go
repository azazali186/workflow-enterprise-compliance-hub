// Package auth handles credentials: JWT issuance/parsing, bcrypt hashing and
// the single-session token fingerprint stored in the cache (mirrors the API
// gateway pattern: only the most recent token per user is valid).
package auth

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/aeroxe/compliance-hub/backend/internal/cache"
)

// TokenCachePrefix is the cache key prefix for active sessions.
const TokenCachePrefix = "auth:token:"

// RenewThreshold: sessions with less remaining time than this are extended.
const RenewThreshold = 30 * time.Minute

// ErrInvalidToken is returned when a token cannot be parsed or verified.
var ErrInvalidToken = errors.New("invalid token")

// Claims is the JWT payload for authenticated users.
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	RoleCode string `json:"role_code"`
	jwt.RegisteredClaims
}

// IssueToken creates a signed HS256 JWT for a user.
func IssueToken(secret string, expiry time.Duration, userID, username, roleCode string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		RoleCode: roleCode,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ParseToken validates a token signature and expiry and returns its claims.
func ParseToken(secret, tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// SessionHash is the single-session fingerprint of a token (md5 of token+user).
func SessionHash(tokenStr, userID string) string {
	sum := md5.Sum([]byte(tokenStr + userID))
	return fmt.Sprintf("%x", sum)
}

// SessionKey returns the cache key for a user's active session.
func SessionKey(userID string) string { return TokenCachePrefix + userID }

// StoreSession records the token fingerprint as the active session.
func StoreSession(ctx context.Context, c cache.Cache, userID, tokenStr string, ttl time.Duration) error {
	return c.Set(ctx, SessionKey(userID), SessionHash(tokenStr, userID), ttl)
}

// RenewIfNeeded extends the session when it is close to expiry.
func RenewIfNeeded(ctx context.Context, c cache.Cache, userID, expectedHash string, ttl time.Duration) {
	if d, ok := c.TTL(ctx, SessionKey(userID)); ok && d < RenewThreshold {
		_ = c.Set(ctx, SessionKey(userID), expectedHash, ttl)
	}
}

// HashPassword bcrypt-hashes a plaintext password.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword verifies a plaintext password against a hash.
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
