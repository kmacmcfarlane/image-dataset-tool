// Package crypto provides AES-256-GCM encryption and decryption helpers.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
)

// KeySize is the required length of an AES-256 key in bytes.
const KeySize = 32

// ErrKeyMissing indicates the secret.key file does not exist.
var ErrKeyMissing = errors.New("secret.key file not found")

// ErrKeyPermissions indicates the secret.key file has incorrect permissions.
var ErrKeyPermissions = errors.New("secret.key must have mode 0600")

// ErrInvalidKeySize indicates the key is not exactly 32 bytes.
var ErrInvalidKeySize = errors.New("encryption key must be exactly 32 bytes")

// ErrDecryptionFailed indicates that decryption failed (wrong key or corrupted data).
var ErrDecryptionFailed = errors.New("decryption failed: wrong key or corrupted ciphertext")

// LoadKey reads and validates the encryption key from the given path.
// It verifies the file exists and has mode 0600.
func LoadKey(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrKeyMissing, path)
		}
		return nil, fmt.Errorf("stat secret.key: %w", err)
	}

	// Check permissions (owner-only read/write).
	mode := info.Mode().Perm()
	if mode != 0600 {
		return nil, fmt.Errorf("%w: got %04o at %s", ErrKeyPermissions, mode, path)
	}

	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read secret.key: %w", err)
	}

	if len(key) != KeySize {
		return nil, fmt.Errorf("%w: got %d bytes", ErrInvalidKeySize, len(key))
	}

	return key, nil
}

// Encrypt encrypts plaintext using AES-256-GCM with the given key.
// Returns nonce || ciphertext.
func Encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext produced by Encrypt using AES-256-GCM.
// Expects nonce || ciphertext format.
func Decrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrDecryptionFailed
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}
