package helpers

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword membuat hash bcrypt dari password polos.
func HashPassword(plain string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hashed), nil
}

// CheckPassword membandingkan password polos dengan hash bcrypt.
func CheckPassword(hashed, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}

// GenerateOpaqueToken membuat token acak 256-bit dalam bentuk hex.
// Dipakai untuk refresh token: nilainya tidak membawa informasi apa pun,
// pasangannya dicari lewat hash di database.
func GenerateOpaqueToken() (string, error) {
	buf := make([]byte, 32)

	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return hex.EncodeToString(buf), nil
}

// HashToken membuat sidik jari SHA-256 dari token. Refresh token cukup panjang
// dan acak, jadi tidak perlu key derivation function seperti bcrypt.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// SecureCompare membandingkan dua string tanpa membocorkan panjang kecocokan
// lewat waktu eksekusi.
func SecureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
