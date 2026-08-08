// Package auth handles credentials: JWT issuance/parsing, bcrypt hashing and
// the single-session token fingerprint stored in the cache (mirrors the API
// gateway pattern: only the most recent token per user is valid).
package auth

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"strconv"
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

// --- account lockout (failed-login protection, Redis-backed) ---

// LockoutCachePrefix keys the per-account failed-attempt counter.
const LockoutCachePrefix = "auth:lockout:"

// LockoutMaxFailures is the failed-password/unknown-user threshold after which
// the account is temporarily locked.
const LockoutMaxFailures = 5

// LockoutWindow is how long a lock (and the counter) lives before resetting.
const LockoutWindow = 15 * time.Minute

// LockoutKey returns the cache key for an account's failure counter.
func LockoutKey(username string) string { return LockoutCachePrefix + username }

// Locked reports whether an account is currently locked out.
func Locked(ctx context.Context, c cache.Cache, username string) bool {
	if v, ok := c.Get(ctx, LockoutKey(username)); ok {
		if n, err := strconv.Atoi(v); err == nil && n >= LockoutMaxFailures {
			return true
		}
	}
	return false
}

// RecordLockoutFailure increments the failure counter (window-reset) and
// returns true when the account crosses into locked-out.
func RecordLockoutFailure(ctx context.Context, c cache.Cache, username string) bool {
	count := 1
	if v, ok := c.Get(ctx, LockoutKey(username)); ok {
		if n, err := strconv.Atoi(v); err == nil {
			count = n + 1
		}
	}
	_ = c.Set(ctx, LockoutKey(username), strconv.Itoa(count), LockoutWindow)
	return count >= LockoutMaxFailures
}

// ClearLockout resets the counter after a successful login.
func ClearLockout(ctx context.Context, c cache.Cache, username string) {
	_ = c.Del(ctx, LockoutKey(username))
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
