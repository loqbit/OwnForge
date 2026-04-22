package lock

import (
	"crypto/rand"
	"encoding/hex"
)

// randomToken generates a 16-byte random token that identifies the lock holder.
func randomToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Failures from crypto/rand are extremely rare; even with an empty string fallback, the lock will still expire by TTL.
		return ""
	}
	return hex.EncodeToString(b)
}
