package crypto

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"golang.org/x/crypto/bcrypt"
)

// otpLength is the fixed length of generated OTPs.
const otpLength = 6

// bcryptCost for OTP hashes. 10 is the library default and is a good balance
// for short-lived OTPs where the hash is checked at most a handful of times.
const bcryptCost = 10

// GenerateOTP returns a 6-digit numeric OTP (zero-padded) generated with
// crypto/rand.
func GenerateOTP() (string, error) {
	// Upper bound is 10^otpLength (exclusive). Each digit is uniformly
	// distributed so the whole code is uniform over [0, 10^6).
	maxVal := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(otpLength)), nil)
	n, err := rand.Int(rand.Reader, maxVal)
	if err != nil {
		return "", fmt.Errorf("crypto.GenerateOTP: %w", err)
	}
	return fmt.Sprintf("%0*d", otpLength, n.Int64()), nil
}

// HashOTP returns a bcrypt hash of the given OTP code.
func HashOTP(otp string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(otp), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("crypto.HashOTP: %w", err)
	}
	return string(hash), nil
}

// VerifyOTP reports whether the provided OTP matches the given bcrypt hash.
// Any error (including mismatch) returns false.
func VerifyOTP(otp, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(otp)) == nil
}
