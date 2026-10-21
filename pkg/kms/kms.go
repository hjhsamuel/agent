package kms

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"
)

type KMS struct {
	lock   sync.RWMutex
	keys   map[int64][]byte
	latest int64
}

func (k *KMS) GenerateKey(f PersistFunc) (int64, error) {
	key := make([]byte, KeyLength)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return 0, err
	}

	k.lock.Lock()
	defer k.lock.Unlock()

	version := k.latest + 1
	if _, ok := k.keys[version]; ok {
		return 0, fmt.Errorf("kms: version %d already exists", version)
	}

	encodedKey := base64.StdEncoding.EncodeToString(key)

	if err := f(version, encodedKey); err != nil {
		return 0, err
	}

	k.keys[version] = key
	k.latest = version

	return version, nil
}

func (k *KMS) GetKey(version int64) (string, error) {
	key, err := k.getKey(version)
	if err != nil {
		return "", err
	}

	encodedKey := base64.StdEncoding.EncodeToString(key)
	return encodedKey, nil
}

func (k *KMS) getKey(version int64) ([]byte, error) {
	k.lock.RLock()
	defer k.lock.RUnlock()

	if v, ok := k.keys[version]; ok {
		return v, nil
	}
	return nil, errors.New("version not set")
}

func (k *KMS) GetLatestKey() (int64, string, error) {
	k.lock.RLock()
	defer k.lock.RUnlock()

	if k.latest == 0 {
		return 0, "", errors.New("no version found")
	}
	if v, ok := k.keys[k.latest]; ok {
		encodedKey := base64.StdEncoding.EncodeToString(v)
		return k.latest, encodedKey, nil
	}

	return 0, "", errors.New("latest key not set")
}

func (k *KMS) SetKey(keys ...*KeySet) error {
	k.lock.Lock()
	defer k.lock.Unlock()

	latest := k.latest
	for _, item := range keys {
		if err := k.setKey(item); err != nil {
			return err
		}
		if item.Version > latest {
			latest = item.Version
		}
	}
	k.latest = latest

	return nil
}

func (k *KMS) setKey(item *KeySet) error {
	key, err := base64.StdEncoding.DecodeString(item.Key)
	if err != nil {
		return err
	}

	if _, ok := k.keys[item.Version]; ok {
		return fmt.Errorf("kms: version %d already exists", item.Version)
	}
	k.keys[item.Version] = key

	return nil
}

func (k *KMS) Decrypt(version int64, ciphertext, nonce []byte) (string, error) {
	key, err := k.getKey(version)
	if err != nil {
		return "", err
	}

	return Decrypt(key, ciphertext, nonce)
}

func (k *KMS) Encrypt(plaintext string) (*EncryptedValue, error) {
	k.lock.RLock()
	version := k.latest
	key, ok := k.keys[k.latest]
	k.lock.RUnlock()
	if !ok {
		return nil, errors.New("kms: key not set")
	}

	val, err := Encrypt(key, plaintext)
	if err != nil {
		return nil, err
	}

	val.Version = version
	return val, nil
}

func NewKMS() *KMS {
	return &KMS{
		keys: make(map[int64][]byte),
	}
}
