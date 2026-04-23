package crypto

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
)

// HashFingerprint returns the hex-encoded SHA-256 digest of the raw device
// fingerprint. The hash is stable and suitable for storage in claims or a
// session record.
func HashFingerprint(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// CompareFingerprint reports whether the raw fingerprint hashes to the given
// hex-encoded hash. Comparison uses constant-time equality to avoid leaking
// bytes of the expected hash through timing.
func CompareFingerprint(raw, hash string) bool {
	candidate := HashFingerprint(raw)
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(hash)) == 1
}
