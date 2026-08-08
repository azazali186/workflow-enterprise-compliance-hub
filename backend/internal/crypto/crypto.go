// Package crypto provides AES-256-GCM authenticated encryption for
// sensitive data at rest. It is used by the Secret model type and by the
// outbox to encrypt event payloads before they are persisted.
//
// Key rotation: ciphertext carries a versioned prefix derived from the
// encrypting key's fingerprint (enc:k<keyID>:), so a KeyRing holding the
// current key plus the previous key(s) can read old rows while new writes
// always use the current key. The re-encryption worker (internal/reencrypt)
// migrates old rows to the current key so the previous key can be retired.
//
// The current key comes from ENCRYPTION_KEY (32 bytes, hex- or
// base64-encoded); previous key(s) from ENCRYPTION_KEY_PREVIOUS (comma
// separated). A development-only fallback key is used when no key is
// configured so the server can boot locally; production config validation
// requires a real current key (see internal/config).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
)

// legacyPrefix is the pre-rotation ciphertext format (single key, no key ID).
// It is still understood on read so rows written before rotation keep working.
const legacyPrefix = "enc:v1:"

// devKey is a documented non-secret fallback so local development works
// without configuration — exactly like the JWT dev-only secret. It must
// never be used in production (config.Load refuses to boot in non-dev
// environments without a real ENCRYPTION_KEY).
var devKey = []byte("0123456789abcdef0123456789abcdef") // 32 bytes

// ErrInvalidKey is returned when a configured key cannot be used.
var ErrInvalidKey = errors.New("crypto: invalid encryption key (want 32 bytes)")

// ErrUnknownKey is returned when ciphertext references a key that is not in
// the ring (e.g. a key was rotated twice and the intermediate key is gone).
var ErrUnknownKey = errors.New("crypto: ciphertext key not in ring")

// Cipher is a single AES-256-GCM encryption context. It remains the low-level
// primitive (and the legacy enc:v1: writer/reader); application code should
// use the package-level ring functions instead.
type Cipher struct {
	aead cipher.AEAD
}

// New builds a Cipher from a raw 32-byte key.
func New(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("%w: got %d bytes", ErrInvalidKey, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// ParseKey decodes a key from hex (64 chars), standard base64 (44 chars) or
// raw 32 bytes.
func ParseKey(encoded string) ([]byte, error) {
	if len(encoded) == 64 {
		if k, err := hex.DecodeString(encoded); err == nil {
			return k, nil
		}
	}
	if len(encoded) == 44 {
		if k, err := base64.StdEncoding.DecodeString(encoded); err == nil && len(k) == 32 {
			return k, nil
		}
	}
	if len(encoded) == 32 {
		return []byte(encoded), nil
	}
	return nil, ErrInvalidKey
}

// Encrypt seals plaintext into the legacy "enc:v1:" format. Kept for the
// legacy reader and for tests; new writes go through KeyRing.Encrypt, which
// records the key fingerprint.
func (c *Cipher) Encrypt(plaintext []byte) (string, error) {
	if c == nil {
		return "", errors.New("crypto: nil cipher")
	}
	sealed, err := seal(c.aead, plaintext)
	if err != nil {
		return "", err
	}
	return legacyPrefix + sealed, nil
}

// Decrypt reads legacy "enc:v1:" ciphertext (and passes plaintext through).
// Versioned "enc:k<keyID>:" ciphertext is refused — Cipher has no key ID and
// would otherwise return ciphertext as if it were plaintext; the ring must be
// used for versioned values.
func (c *Cipher) Decrypt(value string) ([]byte, error) {
	if c == nil {
		return nil, errors.New("crypto: nil cipher")
	}
	if strings.HasPrefix(value, "enc:k") {
		return nil, errors.New("crypto: versioned ciphertext requires the key ring, not Cipher")
	}
	if !strings.HasPrefix(value, legacyPrefix) {
		return []byte(value), nil
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(value, legacyPrefix))
	if err != nil {
		return nil, fmt.Errorf("crypto: corrupt ciphertext: %w", err)
	}
	return open(c.aead, raw)
}

// EncryptString is a convenience wrapper returning a string.
func (c *Cipher) EncryptString(plain string) (string, error) { return c.Encrypt([]byte(plain)) }

// DecryptString is a convenience wrapper returning a string.
func (c *Cipher) DecryptString(enc string) (string, error) {
	b, err := c.Decrypt(enc)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func seal(aead cipher.AEAD, plaintext []byte) (string, error) {
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, plaintext, nil)
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

func open(aead cipher.AEAD, raw []byte) ([]byte, error) {
	if len(raw) < aead.NonceSize() {
		return nil, errors.New("crypto: ciphertext too short")
	}
	nonce, ct := raw[:aead.NonceSize()], raw[aead.NonceSize():]
	out, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: decryption failed (wrong key or tampered data): %w", err)
	}
	return out, nil
}

// --- KeyRing (rotation-aware) ---

// keyIDLength is the hex fingerprint length used in prefixes.
const keyIDLength = 16 // 8 bytes of SHA-256 -> 16 hex chars

// keyEntry pairs a key with its fingerprint ID.
type keyEntry struct {
	id     string
	cipher *Cipher
}

// KeyRing holds the current encryption key plus previous keys for dual-read.
type KeyRing struct {
	current  *keyEntry
	previous []*keyEntry
}

// keyID derives a stable identifier from the raw key bytes.
func keyID(key []byte) string {
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:8])
}

// setup builds a ring from parsed keys without touching the package default.
func setup(current []byte, previous [][]byte) (*KeyRing, error) {
	cur, err := New(current)
	if err != nil {
		return nil, err
	}
	ring := &KeyRing{current: &keyEntry{id: keyID(current), cipher: cur}}
	for _, p := range previous {
		c, err := New(p)
		if err != nil {
			return nil, err
		}
		ring.previous = append(ring.previous, &keyEntry{id: keyID(p), cipher: c})
	}
	return ring, nil
}

// CurrentKeyID returns the fingerprint of the current key (for the worker and
// logs).
func (k *KeyRing) CurrentKeyID() string {
	if k == nil || k.current == nil {
		return ""
	}
	return k.current.id
}

// UsesCurrentKey reports whether a stored value was already encrypted with the
// current key (the re-encryption worker skips these rows).
func (k *KeyRing) UsesCurrentKey(value string) bool {
	if k == nil || k.current == nil {
		return false
	}
	return strings.HasPrefix(value, "enc:k"+k.current.id+":")
}

// Encrypt seals plaintext under the current key and records its fingerprint.
func (k *KeyRing) Encrypt(plaintext []byte) (string, error) {
	if k == nil || k.current == nil {
		return "", errors.New("crypto: nil key ring")
	}
	sealed, err := seal(k.current.cipher.aead, plaintext)
	if err != nil {
		return "", err
	}
	return "enc:k" + k.current.id + ":" + sealed, nil
}

// EncryptString is a convenience wrapper returning a string.
func (k *KeyRing) EncryptString(plain string) (string, error) { return k.Encrypt([]byte(plain)) }

// Decrypt reverses Encrypt with dual-read: the current key is tried first,
// then the previous key(s). Values without an enc: prefix (plaintext) pass
// through unchanged, and legacy enc:v1: ciphertext is tried against every
// ring key in order.
func (k *KeyRing) Decrypt(value string) ([]byte, error) {
	if k == nil || k.current == nil {
		return nil, errors.New("crypto: nil key ring")
	}
	if !strings.HasPrefix(value, "enc:") {
		return []byte(value), nil
	}

	// Versioned format: enc:k<keyID>:<payload>
	if strings.HasPrefix(value, "enc:k") {
		colon := strings.IndexByte(value[len("enc:k"):], ':')
		if colon < 0 {
			return nil, errors.New("crypto: corrupt ciphertext (missing key id)")
		}
		id := value[len("enc:k") : len("enc:k")+colon]
		raw, err := base64.RawStdEncoding.DecodeString(value[len("enc:k")+colon+1:])
		if err != nil {
			return nil, fmt.Errorf("crypto: corrupt ciphertext: %w", err)
		}
		if e := findEntry(k, id); e != nil {
			return open(e.cipher.aead, raw)
		}
		return nil, fmt.Errorf("%w: %s", ErrUnknownKey, id)
	}

	// Legacy format: enc:v1:<payload> — try every ring key.
	if strings.HasPrefix(value, legacyPrefix) {
		raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(value, legacyPrefix))
		if err != nil {
			return nil, fmt.Errorf("crypto: corrupt ciphertext: %w", err)
		}
		var firstErr error
		for _, e := range k.allEntries() {
			out, err := open(e.cipher.aead, raw)
			if err == nil {
				return out, nil
			}
			if firstErr == nil {
				firstErr = err
			}
		}
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, errors.New("crypto: no key in ring matches legacy ciphertext")
	}

	return nil, errors.New("crypto: unrecognized ciphertext prefix")
}

// DecryptString is a convenience wrapper returning a string.
func (k *KeyRing) DecryptString(enc string) (string, error) {
	b, err := k.Decrypt(enc)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (k *KeyRing) allEntries() []*keyEntry {
	out := make([]*keyEntry, 0, 1+len(k.previous))
	out = append(out, k.current)
	out = append(out, k.previous...)
	return out
}

func findEntry(k *KeyRing, id string) *keyEntry {
	if k.current.id == id {
		return k.current
	}
	for _, e := range k.previous {
		if e.id == id {
			return e
		}
	}
	return nil
}

// --- package-level default (used by models.Secret and the outbox) ---

var (
	mu         sync.RWMutex
	defaultKey *KeyRing
)

// Setup installs the package-wide key ring used by models.Secret and the
// outbox. Call it once at boot with the configured ENCRYPTION_KEY as the
// current key and ENCRYPTION_KEY_PREVIOUS entries (if any) for dual-read.
// Keys are accepted in hex, base64 or raw 32-byte form. An empty current key
// falls back to the dev key.
func Setup(current string, previous ...string) error {
	if current == "" {
		// Development fallback, same spirit as the JWT dev secret.
		current = string(devKey)
	}
	cur, err := ParseKey(current)
	if err != nil {
		return err
	}
	var prev [][]byte
	for _, p := range previous {
		if p == "" {
			continue
		}
		k, err := ParseKey(p)
		if err != nil {
			return fmt.Errorf("crypto: previous key: %w", err)
		}
		prev = append(prev, k)
	}
	ring, err := setup(cur, prev)
	if err != nil {
		return err
	}
	mu.Lock()
	defaultKey = ring
	mu.Unlock()
	return nil
}

// SetDefault installs a single-key ring (no dual-read). Kept for callers that
// do not participate in rotation; Setup is preferred.
func SetDefault(encodedKey string) error {
	if encodedKey == "" {
		encodedKey = string(devKey)
	}
	return Setup(encodedKey)
}

// Default returns the installed key ring, lazily initializing with the dev key
// when Setup was never called (unit tests, misconfig).
func Default() *KeyRing {
	mu.RLock()
	r := defaultKey
	mu.RUnlock()
	if r != nil {
		return r
	}
	mu.Lock()
	defer mu.Unlock()
	if defaultKey == nil {
		// Internal setup (no mutex) so the lazy init cannot deadlock.
		if ring, err := setup(devKey, nil); err == nil {
			defaultKey = ring
		}
	}
	return defaultKey
}

// CurrentKeyID reports the fingerprint of the package-wide current key.
func CurrentKeyID() string { return Default().CurrentKeyID() }

// UsesCurrentKey reports whether a value uses the package-wide current key.
func UsesCurrentKey(value string) bool { return Default().UsesCurrentKey(value) }

// EncryptString encrypts via the package default ring.
func EncryptString(plain string) (string, error) { return Default().EncryptString(plain) }

// DecryptString decrypts via the package default ring (dual-read).
func DecryptString(enc string) (string, error) { return Default().DecryptString(enc) }
