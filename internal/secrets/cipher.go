// Package secrets stores credentials encrypted at rest and masks them in
// run output. Values are encrypted with AES-256-GCM under a key kept in the
// OS keychain; the database never holds a plaintext secret.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

const KeySize = 32

type Cipher struct {
	aead cipher.AEAD
}

func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("secret key must be %d bytes, got %d", KeySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// Seal encrypts value. The secret's name is authenticated with it, so a
// ciphertext can't be moved to another name.
func (c *Cipher) Seal(name, value string) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return c.aead.Seal(nonce, nonce, []byte(value), []byte(name)), nil
}

func (c *Cipher) Open(name string, data []byte) (string, error) {
	n := c.aead.NonceSize()
	if len(data) < n {
		return "", errors.New("ciphertext too short")
	}
	plain, err := c.aead.Open(nil, data[:n], data[n:], []byte(name))
	if err != nil {
		return "", fmt.Errorf("decrypt %s: wrong key or corrupted value", name)
	}
	return string(plain), nil
}
