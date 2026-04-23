package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHashFingerprint_Deterministic(t *testing.T) {
	h1 := HashFingerprint("device-abc")
	h2 := HashFingerprint("device-abc")
	assert.Equal(t, h1, h2)
	// SHA-256 hex = 64 chars.
	assert.Len(t, h1, 64)
}

func TestHashFingerprint_DifferentInputs(t *testing.T) {
	assert.NotEqual(t, HashFingerprint("a"), HashFingerprint("b"))
}

func TestCompareFingerprint(t *testing.T) {
	raw := "device-fingerprint-xyz"
	h := HashFingerprint(raw)
	assert.True(t, CompareFingerprint(raw, h))
	assert.False(t, CompareFingerprint("other-device", h))
	assert.False(t, CompareFingerprint(raw, "not-a-hash"))
}
