package auth

import (
	"github.com/alexedwards/argon2id"
)

// argon2Params follows current OWASP argon2id recommendations (2024+).
var argon2Params = &argon2id.Params{
	Memory:      64 * 1024, // 64 MiB
	Iterations:  3,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

// HashPassword hashes a plaintext password using argon2id.
// The returned string self-describes its parameters so future reads
// don't need to know the current params.
func HashPassword(plaintext string) (string, error) {
	return argon2id.CreateHash(plaintext, argon2Params)
}

// VerifyPassword returns true if plaintext matches the stored hash.
func VerifyPassword(plaintext, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(plaintext, hash)
}
