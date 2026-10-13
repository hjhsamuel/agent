package kms

type KeySet struct {
	Version int64
	Key     string
}

type EncryptedValue struct {
	Ciphertext []byte
	Nonce      []byte
}
