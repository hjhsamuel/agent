package kms

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

const (
	KeyLength = 32
)

// ValidateKey requires AES-256 key material.
func ValidateKey(key []byte) error {
	if len(key) != KeyLength {
		return fmt.Errorf("key must be %d bytes", KeyLength)
	}
	return nil
}

func Encrypt(key []byte, plaintext string) (*EncryptedValue, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return &EncryptedValue{
		Ciphertext: ciphertext,
		Nonce:      nonce,
	}, nil
}

func Decrypt(key, ciphertext, nonce []byte) (string, error) {
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(nonce) != gcm.NonceSize() {
		return "", fmt.Errorf("nonce must be %d bytes", gcm.NonceSize())
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
