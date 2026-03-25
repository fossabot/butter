package appkey

import (
	"crypto/rand"
	"strings"
)

const (
	Prefix   = "btr_"
	RandLen  = 20
	TokenLen = len(Prefix) + RandLen

	charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// Generate creates a new random application key with the btr_ prefix.
// Uses crypto/rand for ~119 bits of entropy.
func Generate() (string, error) {
	b := make([]byte, RandLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return Prefix + string(b), nil
}

// IsValid reports whether key has the correct btr_ prefix, length, and
// character set. It does not check whether the key is provisioned.
func IsValid(key string) bool {
	if len(key) != TokenLen {
		return false
	}
	if !strings.HasPrefix(key, Prefix) {
		return false
	}
	for _, c := range key[len(Prefix):] {
		if !isAlphanumeric(c) {
			return false
		}
	}
	return true
}

func isAlphanumeric(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
