// Package security provides DataDeck's credential protection primitives.
//
// Credentials persisted in the embedded store are encrypted with AES-256-GCM
// using Go's standard library cryptography. There is no custom cryptography
// here: key parsing, nonce handling and authenticated encryption all delegate
// to crypto/aes, crypto/cipher and crypto/rand.
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
)

// KeySize is the AES-256 key length in bytes.
const KeySize = 32

// formatVersion prefixes every ciphertext so the on-disk format can be
// migrated deliberately if the algorithm or key handling changes.
const formatVersion byte = 1

// Sentinel errors. Callers match them with errors.Is.
var (
	ErrInvalidKey        = errors.New("security: invalid encryption key")
	ErrInvalidCiphertext = errors.New("security: invalid ciphertext")
)

// Cipher encrypts and decrypts credentials with AES-256-GCM.
type Cipher struct {
	aead cipher.AEAD
}

// NewCipher builds a Cipher from key material.
//
// Accepted forms are exactly 32 bytes of raw key material, or exactly 64
// hexadecimal characters decoding to 32 bytes. Any other input is rejected with
// ErrInvalidKey. The application never generates, derives or persists the key.
func NewCipher(keyMaterial string) (*Cipher, error) {
	key, err := parseKey(keyMaterial)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("security: create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("security: create GCM: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

func parseKey(material string) ([]byte, error) {
	if material == "" {
		return nil, fmt.Errorf("%w: encryption key is not configured", ErrInvalidKey)
	}
	if raw := []byte(material); len(raw) == KeySize {
		return raw, nil
	}
	if len(material) == hex.EncodedLen(KeySize) {
		if key, err := hex.DecodeString(material); err == nil && len(key) == KeySize {
			return key, nil
		}
	}
	return nil, fmt.Errorf("%w: expected %d raw bytes or %d hexadecimal characters",
		ErrInvalidKey, KeySize, hex.EncodedLen(KeySize))
}

// Encrypt seals plaintext with AES-256-GCM and returns
// base64(version || nonce || ciphertext || tag).
//
// A fresh random nonce is generated for every call. The nonce is not secret,
// but reusing one under the same key destroys GCM security, so it is never
// taken from a caller or reused.
func (c *Cipher) Encrypt(plaintext []byte) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("security: generate nonce: %w", err)
	}
	sealed := c.aead.Seal(nil, nonce, plaintext, nil)

	payload := make([]byte, 0, 1+len(nonce)+len(sealed))
	payload = append(payload, formatVersion)
	payload = append(payload, nonce...)
	payload = append(payload, sealed...)

	return base64.StdEncoding.EncodeToString(payload), nil
}

// Decrypt opens a value produced by Encrypt.
//
// Malformed input, unsupported format versions, empty strings and
// authentication failures (tampering or wrong key) all return
// ErrInvalidCiphertext. No plaintext is ever returned on failure, and errors
// never include the input value or any plaintext.
func (c *Cipher) Decrypt(encoded string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: malformed encoding", ErrInvalidCiphertext)
	}

	nonceSize := c.aead.NonceSize()
	if len(raw) < 1+nonceSize+c.aead.Overhead() {
		return nil, fmt.Errorf("%w: value is too short", ErrInvalidCiphertext)
	}
	if raw[0] != formatVersion {
		return nil, fmt.Errorf("%w: unsupported format version", ErrInvalidCiphertext)
	}

	nonce := raw[1 : 1+nonceSize]
	sealed := raw[1+nonceSize:]
	plaintext, err := c.aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: authentication failed", ErrInvalidCiphertext)
	}
	return plaintext, nil
}
