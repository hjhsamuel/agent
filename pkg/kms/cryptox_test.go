package kms

import "testing"

func TestMalformedNonceReturnsError(t *testing.T) {
	if _, err := Decrypt(make([]byte, KeyLength), nil, []byte{1}); err == nil {
		t.Fatal("invalid nonce accepted")
	}
}
