package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateOTP_Format(t *testing.T) {
	// Run a handful to cover the zero-padding path (n < 100000).
	for i := 0; i < 20; i++ {
		code, err := GenerateOTP()
		require.NoError(t, err)
		assert.Len(t, code, 6, "otp must be 6 chars")
		for _, c := range code {
			assert.True(t, c >= '0' && c <= '9', "otp must be all digits, got %q", code)
		}
	}
}

func TestHashOTP_VerifyOTP_RoundTrip(t *testing.T) {
	code := "123456"
	hash, err := HashOTP(code)
	require.NoError(t, err)
	assert.NotEqual(t, code, hash)
	assert.True(t, VerifyOTP(code, hash))
}

func TestVerifyOTP_WrongCode(t *testing.T) {
	hash, err := HashOTP("123456")
	require.NoError(t, err)
	assert.False(t, VerifyOTP("654321", hash))
	assert.False(t, VerifyOTP("", hash))
	assert.False(t, VerifyOTP("123456", "not-a-bcrypt-hash"))
}
