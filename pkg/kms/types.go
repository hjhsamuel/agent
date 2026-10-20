package kms

type KeySet struct {
	Version int64
	Key     string
}

type EncryptedValue struct {
	Version    int64
	Ciphertext []byte
	Nonce      []byte
}
