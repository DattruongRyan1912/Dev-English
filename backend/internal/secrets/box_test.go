package secrets

import (
	"bytes"
	"strings"
	"testing"
)

func TestBoxRoundTripsAndRejectsTampering(t *testing.T) {
	box, err := New(strings.Repeat("deployment-key", 3))
	if err != nil {
		t.Fatal(err)
	}
	plaintext := "sk-deepseek-secret-value"
	ciphertext, nonce, err := box.Seal(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, []byte(plaintext)) {
		t.Fatal("ciphertext contains the plaintext secret")
	}
	decoded, err := box.Open(ciphertext, nonce)
	if err != nil || decoded != plaintext {
		t.Fatalf("secret did not round-trip: %q, %v", decoded, err)
	}
	ciphertext[0] ^= 1
	if _, err := box.Open(ciphertext, nonce); err == nil {
		t.Fatal("tampered ciphertext was accepted")
	}
}
