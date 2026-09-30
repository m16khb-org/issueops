package toolconformance

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// RandomToken generates the correlation token for a live host probe.
func RandomToken() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		sum := sha256.Sum256([]byte(time.Now().UTC().String()))
		return hex.EncodeToString(sum[:16])
	}
	return hex.EncodeToString(value)
}
