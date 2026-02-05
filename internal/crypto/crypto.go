// Package crypto provides encryption/decryption for the varnish store.
// Uses AES-256-GCM for authenticated encryption and Argon2id for key derivation.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/argon2"
)

const (
	// PasswordEnvVar is the environment variable for the encryption password.
	PasswordEnvVar = "VARNISH_PASSWORD"

	// Version is the current encryption format version.
	Version = 1

	// Key derivation parameters (Argon2id)
	argonTime    = 1
	argonMemory  = 64 * 1024 // 64 MB
	argonThreads = 4
	argonKeyLen  = 32 // AES-256

	// Sizes
	saltSize  = 16
	nonceSize = 12 // GCM standard nonce size
)

// MagicBytes identifies encrypted varnish store files.
var MagicBytes = []byte("VARNISH\x00")

// ErrPasswordRequired is returned when VARNISH_PASSWORD is not set.
var ErrPasswordRequired = errors.New("VARNISH_PASSWORD environment variable not set")

// IsEncrypted returns true if data starts with the varnish magic bytes.
func IsEncrypted(data []byte) bool {
	if len(data) < len(MagicBytes) {
		return false
	}
	for i, b := range MagicBytes {
		if data[i] != b {
			return false
		}
	}
	return true
}

// GetPassword reads the encryption password from VARNISH_PASSWORD env var.
// Returns ErrPasswordRequired if the variable is not set or empty.
func GetPassword() (string, error) {
	password := os.Getenv(PasswordEnvVar)
	if password == "" {
		return "", ErrPasswordRequired
	}
	return password, nil
}

// DeriveKey derives a 256-bit key from password and salt using Argon2id.
func DeriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
}

// Encrypt encrypts plaintext using AES-256-GCM with a key derived from password.
// Returns encrypted data in format: Magic (8B) | Version (1B) | Salt (16B) | Nonce (12B) | Ciphertext+Tag
func Encrypt(plaintext []byte, password string) ([]byte, error) {
	// Generate random salt
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}

	// Derive key
	key := DeriveKey(password, salt)

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Encrypt
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Build output: Magic | Version | Salt | Nonce | Ciphertext
	headerSize := len(MagicBytes) + 1 + saltSize + nonceSize
	result := make([]byte, headerSize+len(ciphertext))

	offset := 0
	copy(result[offset:], MagicBytes)
	offset += len(MagicBytes)

	result[offset] = Version
	offset++

	copy(result[offset:], salt)
	offset += saltSize

	copy(result[offset:], nonce)
	offset += nonceSize

	copy(result[offset:], ciphertext)

	return result, nil
}

// Decrypt decrypts data that was encrypted with Encrypt.
// Returns ErrPasswordRequired if password is empty.
func Decrypt(data []byte, password string) ([]byte, error) {
	if password == "" {
		return nil, ErrPasswordRequired
	}

	// Minimum size: Magic + Version + Salt + Nonce + at least 16 bytes (GCM tag)
	minSize := len(MagicBytes) + 1 + saltSize + nonceSize + 16
	if len(data) < minSize {
		return nil, errors.New("encrypted data too short")
	}

	// Verify magic bytes
	if !IsEncrypted(data) {
		return nil, errors.New("invalid encrypted data: missing magic bytes")
	}

	offset := len(MagicBytes)

	// Check version
	version := data[offset]
	if version != Version {
		return nil, fmt.Errorf("unsupported encryption version: %d", version)
	}
	offset++

	// Extract salt
	salt := data[offset : offset+saltSize]
	offset += saltSize

	// Extract nonce
	nonce := data[offset : offset+nonceSize]
	offset += nonceSize

	// Extract ciphertext
	ciphertext := data[offset:]

	// Derive key
	key := DeriveKey(password, salt)

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}
