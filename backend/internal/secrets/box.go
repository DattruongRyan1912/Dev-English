package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

var ErrNotConfigured = errors.New("secret encryption is not configured")

// Box encrypts provider secrets before they cross the persistence boundary.
// The key is derived once from a deployment-only secret and is never returned
// by any HTTP handler.
type Box struct {
	key [32]byte
}

func New(rawKey string) (*Box, error) {
	rawKey = strings.TrimSpace(rawKey)
	if len(rawKey) < 32 {
		return nil, fmt.Errorf("secret encryption key must be at least 32 characters")
	}
	return &Box{key: sha256.Sum256([]byte(rawKey))}, nil
}

func NewFromEnv() (*Box, error) {
	rawKey := strings.TrimSpace(os.Getenv("DEVENGLISH_SECRET_ENCRYPTION_KEY"))
	if rawKey == "" {
		return nil, ErrNotConfigured
	}
	return New(rawKey)
}

func (b *Box) Seal(plaintext string) (ciphertext, nonce []byte, err error) {
	if b == nil {
		return nil, nil, ErrNotConfigured
	}
	block, err := aes.NewCipher(b.key[:])
	if err != nil {
		return nil, nil, fmt.Errorf("create secret cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("create secret AEAD: %w", err)
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("generate secret nonce: %w", err)
	}
	ciphertext = gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return ciphertext, nonce, nil
}

func (b *Box) Open(ciphertext, nonce []byte) (string, error) {
	if b == nil {
		return "", ErrNotConfigured
	}
	block, err := aes.NewCipher(b.key[:])
	if err != nil {
		return "", fmt.Errorf("create secret cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create secret AEAD: %w", err)
	}
	if len(nonce) != gcm.NonceSize() {
		return "", errors.New("invalid secret nonce")
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("secret decryption failed")
	}
	return string(plaintext), nil
}
