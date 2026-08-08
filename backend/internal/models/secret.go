// Secret is an encrypted-at-rest string column backed by AES-256-GCM
// (internal/crypto). The value is transparently encrypted when written and
// decrypted when read, so application code and API responses always see the
// plaintext while the database stores only ciphertext.
//
// The key is the package-wide default configured at boot from ENCRYPTION_KEY.
// Values that do not carry the "enc:v1:" prefix (legacy or dev rows) are
// passed through unchanged so existing data keeps working.
package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/aeroxe/compliance-hub/backend/internal/crypto"
)

// Secret holds a single encrypted string value.
type Secret struct {
	plain string
}

// NewSecret wraps a plaintext string.
func NewSecret(plain string) Secret { return Secret{plain: plain} }

// String returns the plaintext (already decrypted by Scan).
func (s Secret) String() string { return s.plain }

// Value implements driver.Valuer: encrypts before persistence.
func (s Secret) Value() (driver.Value, error) {
	if s.plain == "" {
		return "", nil
	}
	enc, err := crypto.EncryptString(s.plain)
	if err != nil {
		return nil, fmt.Errorf("secret: encrypt: %w", err)
	}
	return enc, nil
}

// Scan implements sql.Scanner: decrypts after reading.
func (s *Secret) Scan(src any) error {
	var raw string
	switch v := src.(type) {
	case nil:
		s.plain = ""
		return nil
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		return fmt.Errorf("secret: unsupported scan type %T", src)
	}
	plain, err := crypto.DecryptString(raw)
	if err != nil {
		return fmt.Errorf("secret: decrypt: %w", err)
	}
	s.plain = plain
	return nil
}

// MarshalJSON exposes the plaintext to authorized API consumers.
func (s Secret) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.plain)
}

// UnmarshalJSON accepts a plaintext string (encryption happens on write).
func (s *Secret) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &s.plain)
}
