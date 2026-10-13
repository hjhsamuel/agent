package kms

import (
	"errors"
	"sync"
)

type KMS struct {
	lock   sync.RWMutex
	keys   map[int64]string
	latest int64
}

func (k *KMS) GetKey(version int64) (string, error) {
	k.lock.RLock()
	defer k.lock.RUnlock()

	if v, ok := k.keys[version]; ok {
		return v, nil
	}
	return "", errors.New("version not set")
}

func (k *KMS) GetLatestKey() (string, error) {
	k.lock.RLock()
	defer k.lock.RUnlock()

	if k.latest == 0 {
		return "", errors.New("no version found")
	}
	if v, ok := k.keys[k.latest]; ok {
		return v, nil
	}

	return "", errors.New("latest key not set")
}

func (k *KMS) SetKey(keys ...*KeySet) {
	k.lock.Lock()
	defer k.lock.Unlock()

	latest := k.latest
	for _, item := range keys {
		k.keys[item.Version] = item.Key
		if item.Version > latest {
			latest = item.Version
		}
	}
	k.latest = latest
}

func (k *KMS) Decrypt(version int64, ciphertext, nonce []byte) (string, error) {
	key, err := k.GetKey(version)
	if err != nil {
		return "", err
	}

	return Decrypt([]byte(key), ciphertext, nonce)
}

func (k *KMS) Encrypt(plaintext string) (*EncryptedValue, error) {
	key, err := k.GetLatestKey()
	if err != nil {
		return nil, err
	}

	return Encrypt([]byte(key), plaintext)
}

func NewKMS() *KMS {
	return &KMS{
		keys: make(map[int64]string),
	}
}
