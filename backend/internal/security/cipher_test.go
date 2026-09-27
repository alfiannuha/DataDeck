package security

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

const testKey = "0123456789abcdef0123456789abcdef" // 32 bytes, test-only

func newTestCipher(t *testing.T) *Cipher {
	t.Helper()
	c, err := NewCipher(testKey)
	if err != nil {
		t.Fatalf("NewCipher() error = %v", err)
	}
	return c
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	c := newTestCipher(t)
	plaintext := []byte("correct horse battery staple")

	encoded, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if encoded == "" {
		t.Fatal("Encrypt() returned empty ciphertext")
	}
	if strings.Contains(encoded, string(plaintext)) {
		t.Error("ciphertext appears to contain the plaintext")
	}

	got, err := c.Decrypt(encoded)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if string(got) != string(plaintext) {
		t.Errorf("Decrypt() = %q, want %q", got, plaintext)
	}
}

func TestEncryptUsesFreshNonce(t *testing.T) {
	c := newTestCipher(t)
	plaintext := []byte("same input")

	first, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() first error = %v", err)
	}
	second, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() second error = %v", err)
	}
	if first == second {
		t.Error("two encryptions of the same plaintext produced identical ciphertext (nonce reuse)")
	}

	firstRaw, _ := base64.StdEncoding.DecodeString(first)
	secondRaw, _ := base64.StdEncoding.DecodeString(second)
	if firstRaw[0] != formatVersion || secondRaw[0] != formatVersion {
		t.Errorf("ciphertext missing format version byte")
	}

	if string(firstRaw[1:1+c.aead.NonceSize()]) == string(secondRaw[1:1+c.aead.NonceSize()]) {
		t.Error("nonce was reused across encryptions")
	}
}

func TestEncryptEmptyPlaintextRoundTrips(t *testing.T) {
	c := newTestCipher(t)

	encoded, err := c.Encrypt(nil)
	if err != nil {
		t.Fatalf("Encrypt(nil) error = %v", err)
	}
	got, err := c.Decrypt(encoded)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Decrypt() = %q, want empty", got)
	}
}

func TestNewCipherRejectsInvalidKeys(t *testing.T) {
	cases := map[string]string{
		"empty":           "",
		"too short":       "short-key",
		"too long":        strings.Repeat("a", 33),
		"invalid hex len": strings.Repeat("z", 64),
	}
	for name, key := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewCipher(key); !errors.Is(err, ErrInvalidKey) {
				t.Errorf("NewCipher(%q) error = %v, want ErrInvalidKey", key, err)
			}
		})
	}
}

func TestNewCipherAcceptsHexKey(t *testing.T) {
	hexKey := "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff" // 64 hex chars
	c, err := NewCipher(hexKey)
	if err != nil {
		t.Fatalf("NewCipher(hex) error = %v", err)
	}

	encoded, err := c.Encrypt([]byte("secret"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	got, err := c.Decrypt(encoded)
	if err != nil || string(got) != "secret" {
		t.Fatalf("round trip with hex key failed: got=%q err=%v", got, err)
	}
}

func TestDecryptRejectsMalformedCiphertext(t *testing.T) {
	c := newTestCipher(t)

	cases := map[string]string{
		"empty":       "",
		"not base64":  "not valid base64!!!",
		"too short":   base64.StdEncoding.EncodeToString([]byte{formatVersion, 1, 2}),
		"bad version": base64.StdEncoding.EncodeToString(append([]byte{99}, make([]byte, 40)...)),
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := c.Decrypt(value); !errors.Is(err, ErrInvalidCiphertext) {
				t.Errorf("Decrypt(%q) error = %v, want ErrInvalidCiphertext", value, err)
			}
		})
	}
}

func TestDecryptDetectsTampering(t *testing.T) {
	c := newTestCipher(t)
	plaintext := []byte("tamper-evident")

	encoded, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	raw[len(raw)-1] ^= 0xff
	tampered := base64.StdEncoding.EncodeToString(raw)

	if _, err := c.Decrypt(tampered); !errors.Is(err, ErrInvalidCiphertext) {
		t.Errorf("Decrypt(tampered) error = %v, want ErrInvalidCiphertext", err)
	}
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	encoder := newTestCipher(t)
	other, err := NewCipher("fedcba9876543210fedcba9876543210")
	if err != nil {
		t.Fatalf("NewCipher(other) error = %v", err)
	}

	encoded, err := encoder.Encrypt([]byte("secret"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if _, err := other.Decrypt(encoded); !errors.Is(err, ErrInvalidCiphertext) {
		t.Errorf("Decrypt with wrong key error = %v, want ErrInvalidCiphertext", err)
	}
}

func TestDecryptErrorDoesNotLeakPlaintext(t *testing.T) {
	c := newTestCipher(t)
	plaintext := "super-secret-credential"

	encoded, err := c.Encrypt([]byte(plaintext))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	raw, _ := base64.StdEncoding.DecodeString(encoded)
	raw[len(raw)-1] ^= 0xff

	_, err = c.Decrypt(base64.StdEncoding.EncodeToString(raw))
	if err == nil {
		t.Fatal("Decrypt() error = nil, want failure")
	}
	if strings.Contains(err.Error(), plaintext) {
		t.Errorf("error message leaked plaintext: %v", err)
	}
}

func TestInvalidKeyErrorDoesNotEchoKeyMaterial(t *testing.T) {
	material := "supersecretkeymaterial-value"
	if _, err := NewCipher(material); err == nil {
		t.Fatal("NewCipher() error = nil, want failure")
	} else if strings.Contains(err.Error(), material) {
		t.Errorf("key error echoed the key material: %v", err)
	}
}

func TestCipherIsKeyStableAcrossInstances(t *testing.T) {
	first := newTestCipher(t)
	second, err := NewCipher(testKey)
	if err != nil {
		t.Fatalf("NewCipher() error = %v", err)
	}
	encoded, err := first.Encrypt([]byte("survives-restart"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	got, err := second.Decrypt(encoded)
	if err != nil || string(got) != "survives-restart" {
		t.Fatalf("decrypt across instances = %q, err=%v", got, err)
	}
}

func TestDecryptNeverReturnsPlaintextOnFailure(t *testing.T) {
	c := newTestCipher(t)
	encoded, err := c.Encrypt([]byte("value"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	raw, _ := base64.StdEncoding.DecodeString(encoded)
	for i := range raw {
		tampered := append([]byte(nil), raw...)
		tampered[i] ^= 0x01
		got, err := c.Decrypt(base64.StdEncoding.EncodeToString(tampered))
		if err == nil {
			t.Fatalf("tampered byte %d accepted", i)
		}
		if len(got) != 0 {
			t.Fatalf("tampered byte %d returned %d bytes of plaintext on failure", i, len(got))
		}
	}
}
