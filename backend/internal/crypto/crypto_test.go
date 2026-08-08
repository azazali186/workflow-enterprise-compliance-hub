package crypto

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

const testKeyHex = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

func newTestCipher(t *testing.T) *Cipher {
	t.Helper()
	key, err := hex.DecodeString(testKeyHex)
	if err != nil {
		t.Fatalf("decode key: %v", err)
	}
	c, err := New(key)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestRoundtrip(t *testing.T) {
	c := newTestCipher(t)
	plain := `{"name":"GDPR","metadata":{"owner":"alice"}}`

	enc, err := c.EncryptString(plain)
	if err != nil {
		t.Fatalf("EncryptString: %v", err)
	}
	if !strings.HasPrefix(enc, legacyPrefix) {
		t.Fatalf("ciphertext missing prefix: %q", enc[:min(12, len(enc))])
	}
	if strings.Contains(enc, "GDPR") {
		t.Fatal("ciphertext leaks plaintext")
	}

	got, err := c.DecryptString(enc)
	if err != nil {
		t.Fatalf("DecryptString: %v", err)
	}
	if got != plain {
		t.Errorf("roundtrip = %q, want %q", got, plain)
	}
}

func TestUniqueCiphertextPerCall(t *testing.T) {
	c := newTestCipher(t)
	a, _ := c.EncryptString("same-value")
	b, _ := c.EncryptString("same-value")
	if a == b {
		t.Error("two encryptions of the same value must differ (random nonce)")
	}
}

func TestWrongKeyFails(t *testing.T) {
	c := newTestCipher(t)
	enc, err := c.EncryptString("secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	other, err := New([]byte(strings.Repeat("f", 32)))
	if err != nil {
		t.Fatalf("New(other): %v", err)
	}
	if _, err := other.DecryptString(enc); err == nil {
		t.Fatal("decryption with a different key must fail")
	}
}

func TestTamperDetected(t *testing.T) {
	c := newTestCipher(t)
	enc, _ := c.EncryptString("secret")
	tampered := enc[:len(enc)-4] + "AAAA"
	if _, err := c.DecryptString(tampered); err == nil {
		t.Fatal("tampered ciphertext must fail authentication (GCM tag)")
	}
}

func TestPlaintextPassthrough(t *testing.T) {
	c := newTestCipher(t)
	got, err := c.DecryptString("legacy-plaintext")
	if err != nil {
		t.Fatalf("passthrough: %v", err)
	}
	if got != "legacy-plaintext" {
		t.Errorf("passthrough = %q, want unchanged", got)
	}
}

func TestParseKeyFormats(t *testing.T) {
	raw := []byte("0123456789abcdef0123456789abcdef")

	hexKey, err := ParseKey(hex.EncodeToString(raw))
	if err != nil || string(hexKey) != string(raw) {
		t.Errorf("hex key: %v", err)
	}
	b64Key, err := ParseKey(base64.StdEncoding.EncodeToString(raw))
	if err != nil || string(b64Key) != string(raw) {
		t.Errorf("base64 key: %v", err)
	}
	rawKey, err := ParseKey(string(raw))
	if err != nil || string(rawKey) != string(raw) {
		t.Errorf("raw key: %v", err)
	}
	if _, err := ParseKey("too-short"); err == nil {
		t.Error("invalid key length must fail")
	}
}

func TestDefaultLazilyInitializes(t *testing.T) {
	// Reset the package default to prove lazy init with the dev key works.
	mu.Lock()
	defaultKey = nil
	mu.Unlock()

	enc, err := EncryptString("hello")
	if err != nil {
		t.Fatalf("EncryptString(default): %v", err)
	}
	got, err := DecryptString(enc)
	if err != nil || got != "hello" {
		t.Errorf("default roundtrip = %q, %v", got, err)
	}
}

func TestSetDefaultCustomKey(t *testing.T) {
	if err := SetDefault(testKeyHex); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	enc, _ := EncryptString("value")
	if got, err := DecryptString(enc); err != nil || got != "value" {
		t.Errorf("custom-key roundtrip = %q, %v", got, err)
	}
	// Restore for other tests.
	if err := SetDefault(""); err != nil {
		t.Fatalf("restore default: %v", err)
	}
}

// keyHex returns the hex form of a raw 32-byte key.
func keyHex(seed byte) string {
	b := make([]byte, 32)
	for i := range b {
		b[i] = seed + byte(i)
	}
	return hex.EncodeToString(b)
}

func TestEncryptWritesVersionedPrefix(t *testing.T) {
	old := keyHex(0x10)
	if err := Setup(old); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	enc, err := EncryptString("hello")
	if err != nil {
		t.Fatalf("EncryptString: %v", err)
	}
	wantPrefix := "enc:k" + CurrentKeyID() + ":"
	if !strings.HasPrefix(enc, wantPrefix) {
		t.Errorf("ciphertext %q missing current-key prefix %q", enc[:min(len(enc), 24)], wantPrefix)
	}
	if !UsesCurrentKey(enc) {
		t.Error("UsesCurrentKey(enc) = false, want true")
	}
	if UsesCurrentKey("enc:k0123456789abcdef:xxx") {
		t.Error("UsesCurrentKey(other key) = true, want false")
	}
	// Restore.
	_ = SetDefault("")
}

func TestKeyRotationDualRead(t *testing.T) {
	oldKey, newKey := keyHex(0x20), keyHex(0x30)

	// Data written while the OLD key was current.
	if err := Setup(oldKey); err != nil {
		t.Fatalf("Setup(old): %v", err)
	}
	oldEnc, err := EncryptString("secret-value")
	if err != nil {
		t.Fatalf("encrypt with old key: %v", err)
	}
	oldID := CurrentKeyID()

	// Rotate: new key becomes current, old key moves to previous.
	if err := Setup(newKey, oldKey); err != nil {
		t.Fatalf("Setup(new, old): %v", err)
	}
	if CurrentKeyID() == oldID {
		t.Fatal("current key id did not change after rotation")
	}

	// Old rows still decrypt (dual-read), new writes use the new key.
	if got, err := DecryptString(oldEnc); err != nil || got != "secret-value" {
		t.Errorf("dual-read decrypt = %q, %v", got, err)
	}
	newEnc, err := EncryptString("fresh")
	if err != nil {
		t.Fatalf("encrypt with new key: %v", err)
	}
	if got, err := DecryptString(newEnc); err != nil || got != "fresh" {
		t.Errorf("new-key roundtrip = %q, %v", got, err)
	}
	if UsesCurrentKey(oldEnc) {
		t.Error("old ciphertext marked as current-key — rotation worker would skip it")
	}
	if !UsesCurrentKey(newEnc) {
		t.Error("new ciphertext not marked as current-key")
	}
	// Restore.
	_ = SetDefault("")
}

func TestLegacyPrefixReadsWithPreviousKey(t *testing.T) {
	oldKey, newKey := keyHex(0x40), keyHex(0x50)
	k, err := ParseKey(oldKey)
	if err != nil {
		t.Fatalf("ParseKey: %v", err)
	}
	legacyCipher, err := New(k) // writes the legacy enc:v1: format
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	legacy, err := legacyCipher.EncryptString("pre-rotation-row")
	if err != nil {
		t.Fatalf("legacy encrypt: %v", err)
	}
	if !strings.HasPrefix(legacy, "enc:v1:") {
		t.Fatalf("legacy ciphertext %q missing enc:v1: prefix", legacy)
	}

	if err := Setup(newKey, oldKey); err != nil {
		t.Fatalf("Setup(new, old): %v", err)
	}
	if got, err := DecryptString(legacy); err != nil || got != "pre-rotation-row" {
		t.Errorf("legacy decrypt via previous key = %q, %v", got, err)
	}
	// Restore.
	_ = SetDefault("")
}

func TestDecryptUnknownKeyFails(t *testing.T) {
	third := keyHex(0x60)
	if err := Setup(third); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	enc, _ := EncryptString("orphaned")

	// Ring no longer contains the key that wrote it => hard failure, no silent
	// plaintext fallthrough.
	if err := Setup(keyHex(0x70), keyHex(0x80)); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if _, err := DecryptString(enc); err == nil {
		t.Fatal("decrypt of orphaned ciphertext must fail, not return plaintext")
	}
	// Restore.
	_ = SetDefault("")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
