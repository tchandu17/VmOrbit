// Package crypto provides AES-256-GCM encryption helpers for provider credentials.
// The encryption key is loaded from the VMORBIT_ENCRYPTION_KEY environment variable
// (hex-encoded 32-byte key). If not set, a deterministic fallback is used for
// development — this MUST be replaced before production deployment.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sync"
)

var (
	keyOnce sync.Once
	aesKey  []byte
)

// loadKey returns the 32-byte AES key.
// Priority:
//  1. VMORBIT_ENCRYPTION_KEY env var (hex-encoded 32 bytes = 64 hex chars)
//  2. Development fallback (logs a warning)
func loadKey() []byte {
	keyOnce.Do(func() {
		if raw := os.Getenv("VMORBIT_ENCRYPTION_KEY"); raw != "" {
			decoded, err := hex.DecodeString(raw)
			if err != nil || len(decoded) != 32 {
				panic(fmt.Sprintf(
					"VMORBIT_ENCRYPTION_KEY must be a 64-character hex string (32 bytes). Got %d bytes after decode. Error: %v",
					len(decoded), err,
				))
			}
			aesKey = decoded
			return
		}

		// Development fallback — deterministic, NOT secure for production.
		// The application will log a warning on startup when this path is taken.
		aesKey = []byte("vmOrbit-secret-key-32bytes-pad!!")
	})
	return aesKey
}

// IsDevelopmentKey returns true when the fallback development key is in use.
// Use this to emit a startup warning.
func IsDevelopmentKey() bool {
	return os.Getenv("VMORBIT_ENCRYPTION_KEY") == ""
}

// EncryptSecret encrypts a plaintext secret using AES-256-GCM.
// The output is base64-encoded: nonce (12 bytes) || ciphertext || GCM tag.
func EncryptSecret(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}

	block, err := aes.NewCipher(loadKey())
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptSecret decrypts a base64-encoded AES-256-GCM ciphertext.
func DecryptSecret(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	block, err := aes.NewCipher(loadKey())
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypting: %w", err)
	}

	return string(plain), nil
}

// ReEncryptSecret re-encrypts a value with the current key.
// Used for key rotation: decrypt with old key, re-encrypt with new key.
// The caller is responsible for providing the old-key decrypt function.
func ReEncryptSecret(encoded string, decryptWithOldKey func(string) (string, error)) (string, error) {
	plain, err := decryptWithOldKey(encoded)
	if err != nil {
		return "", fmt.Errorf("decrypt with old key: %w", err)
	}
	return EncryptSecret(plain)
}
