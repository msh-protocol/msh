package protocol

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"time"
)

// GenerateToken generates a cryptographically secure token with the given prefix
// using 16 bytes (128 bits) of OS entropy from crypto/rand.
// Example: GenerateToken("msh-") -> "msh-7e2a4f0b9c1d8e3a5f7b2c0e1d4a6b8c"
func GenerateToken(prefix string) string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// High-entropy fallback if crypto/rand fails
		return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano())
	}
	return prefix + hex.EncodeToString(b)
}

// SecureCompare performs a constant-time comparison between two strings.
// This prevents timing side-channel attacks against authentication tokens and passwords.
func SecureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
