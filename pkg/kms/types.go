package kms

type PersistFunc func(version int64, key string) error

type KeySet struct {
	Version int64
	Key     string
}

type EncryptedValue struct {
	Version    int64
	Ciphertext []byte
	Nonce      []byte
}
