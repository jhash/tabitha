package auth

import (
	"encoding/base64"
	"testing"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	// 32 raw bytes, base64-encoded — matches how TOKEN_ENCRYPTION_KEY is
	// documented in .env.example (`openssl rand -base64 32`).
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}

func TestEncryptDecryptRoundTrips(t *testing.T) {
	key := testKey(t)
	plaintext := "ya29.this-looks-like-a-real-google-access-token"

	ciphertext, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if string(ciphertext) == plaintext {
		t.Fatal("ciphertext equals plaintext — not actually encrypted")
	}

	got, err := Decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if got != plaintext {
		t.Errorf("Decrypt() = %q, want %q", got, plaintext)
	}
}

func TestEncryptProducesDifferentCiphertextEachTime(t *testing.T) {
	// A fresh random nonce per call — same plaintext must not produce
	// identical ciphertext, or patterns in stored tokens would leak.
	key := testKey(t)
	a, err := Encrypt(key, "same-plaintext")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	b, err := Encrypt(key, "same-plaintext")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if string(a) == string(b) {
		t.Error("two Encrypt() calls with the same plaintext produced identical ciphertext")
	}
}

func TestDecryptFailsWithWrongKey(t *testing.T) {
	key := testKey(t)
	wrongKey := make([]byte, 32)
	copy(wrongKey, key)
	wrongKey[0] ^= 0xFF

	ciphertext, err := Encrypt(key, "secret")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if _, err := Decrypt(wrongKey, ciphertext); err == nil {
		t.Error("Decrypt() with the wrong key succeeded, want an error")
	}
}

func TestDecryptFailsOnTamperedCiphertext(t *testing.T) {
	key := testKey(t)
	ciphertext, err := Encrypt(key, "secret")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	tampered := append([]byte(nil), ciphertext...)
	tampered[len(tampered)-1] ^= 0xFF

	if _, err := Decrypt(key, tampered); err == nil {
		t.Error("Decrypt() of tampered ciphertext succeeded, want an error (GCM auth tag should fail)")
	}
}

func TestParseEncryptionKeyDecodesBase64(t *testing.T) {
	raw := testKey(t)
	encoded := base64.StdEncoding.EncodeToString(raw)

	key, err := ParseEncryptionKey(encoded)
	if err != nil {
		t.Fatalf("ParseEncryptionKey() error = %v", err)
	}
	if string(key) != string(raw) {
		t.Error("ParseEncryptionKey() did not decode to the original key bytes")
	}
}

func TestParseEncryptionKeyRejectsWrongLength(t *testing.T) {
	short := base64.StdEncoding.EncodeToString([]byte("too-short"))
	if _, err := ParseEncryptionKey(short); err == nil {
		t.Error("expected an error for a key that isn't 32 bytes after decoding")
	}
}
