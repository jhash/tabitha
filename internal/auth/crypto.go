// Package auth handles superadmin authentication: Google OAuth (goth),
// app-level sessions (Postgres-backed), and at-rest encryption of the
// stored Drive/Docs OAuth token used by the ingestion jobs.
package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

const keySize = 32 // AES-256

// ParseEncryptionKey decodes TOKEN_ENCRYPTION_KEY (base64, e.g. from
// `openssl rand -base64 32`) into raw key bytes.
func ParseEncryptionKey(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("auth: decoding encryption key: %w", err)
	}
	if len(key) != keySize {
		return nil, fmt.Errorf("auth: encryption key must decode to %d bytes, got %d", keySize, len(key))
	}
	return key, nil
}

// Encrypt seals plaintext with AES-256-GCM, prepending the nonce so Decrypt
// is self-contained. Never store OAuth tokens as plaintext.
func Encrypt(key []byte, plaintext string) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("auth: generating nonce: %w", err)
	}

	return gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// Decrypt reverses Encrypt. Fails if the key is wrong or the ciphertext was
// tampered with (GCM's authentication tag check).
func Decrypt(key []byte, ciphertext []byte) (string, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("auth: ciphertext shorter than nonce size")
	}
	nonce, sealed := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("auth: decrypting: %w", err)
	}
	return string(plaintext), nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("auth: creating cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("auth: creating GCM: %w", err)
	}
	return gcm, nil
}
