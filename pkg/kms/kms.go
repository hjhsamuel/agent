package kms

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"sync"
)

type KMS struct {
	lock   sync.RWMutex
	keys   map[int64]string
	latest int64
}

func (k *KMS) GenerateKey() (int64, string, error) {
	key := make([]byte, KeyLength)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return 0, "", err
	}

	k.lock.Lock()
	defer k.lock.Unlock()

	version := k.latest + 1
	err := k.SetKey(&KeySet{
		Version: version,
		Key:     string(key),
	})
	if err != nil {
		return 0, "", err
	}

	return version, string(key), nil
}

func (k *KMS) GetKey(version int64) (string, error) {
	k.lock.RLock()
	defer k.lock.RUnlock()

	if v, ok := k.keys[version]; ok {
		return v, nil
	}
	return "", errors.New("version not set")
}

func (k *KMS) GetLatestKey() (int64, string, error) {
	k.lock.RLock()
	defer k.lock.RUnlock()

	if k.latest == 0 {
		return 0, "", errors.New("no version found")
	}
	if v, ok := k.keys[k.latest]; ok {
		return k.latest, v, nil
	}

	return 0, "", errors.New("latest key not set")
}

func (k *KMS) SetKey(keys ...*KeySet) error {
	k.lock.Lock()
	defer k.lock.Unlock()

	latest := k.latest
	for _, item := range keys {
		if _, ok := k.keys[item.Version]; ok {
			return fmt.Errorf("kms: version %d already exists", item.Version)
		}
		k.keys[item.Version] = item.Key
		if item.Version > latest {
			latest = item.Version
		}
	}
	k.latest = latest

	return nil
}

func (k *KMS) Decrypt(version int64, ciphertext, nonce []byte) (string, error) {
	key, err := k.GetKey(version)
	if err != nil {
		return "", err
	}

	return Decrypt([]byte(key), ciphertext, nonce)
}

func (k *KMS) Encrypt(plaintext string) (*EncryptedValue, error) {
	version, key, err := k.GetLatestKey()
	if err != nil {
		return nil, err
	}

	val, err := Encrypt([]byte(key), plaintext)
	if err != nil {
		return nil, err
	}

	val.Version = version
	return val, nil
}

func NewKMS() *KMS {
	return &KMS{
		keys: make(map[int64]string),
	}
}
