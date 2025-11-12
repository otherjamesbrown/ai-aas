// Package security provides MFA (Multi-Factor Authentication) verification utilities.
//
// Purpose:
//   This package implements TOTP (Time-based One-Time Password) verification
//   for MFA-enabled users. It validates TOTP codes against stored secrets using
//   the RFC 6238 standard.
//
// Dependencies:
//   - github.com/pquerna/otp: TOTP code generation and verification
//
// Key Responsibilities:
//   - VerifyTOTP: Validates a TOTP code against a secret
//   - GenerateTOTPSecret: Generates a new TOTP secret for enrollment
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#FR-001 (User Authentication with MFA)
//
// Debugging Notes:
//   - TOTP codes are valid for 30-second windows (standard)
//   - Verification allows ±1 time step tolerance for clock skew
//   - Secrets are base32-encoded strings (standard TOTP format)
//
// Thread Safety:
//   - All functions are stateless and safe for concurrent use
//
// Error Handling:
//   - Invalid secret format returns error
//   - Invalid code format returns false (not an error)
//   - Clock skew issues handled by tolerance window
package security

import (
	"encoding/base32"
	"fmt"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// VerifyTOTP validates a TOTP code against a secret.
// Returns true if the code is valid, false otherwise.
// Uses standard TOTP parameters: 30-second window, SHA1 hash, 6 digits.
func VerifyTOTP(secret, code string) (bool, error) {
	if secret == "" || code == "" {
		return false, nil
	}

	// Validate secret is valid base32
	_, err := base32.StdEncoding.DecodeString(secret)
	if err != nil {
		return false, fmt.Errorf("invalid TOTP secret format: %w", err)
	}

	// Verify code with ±1 time step tolerance for clock skew
	valid := totp.Validate(code, secret)
	return valid, nil
}

// GenerateTOTPSecret generates a new TOTP secret for user enrollment.
// Returns a base32-encoded secret string suitable for QR code generation.
func GenerateTOTPSecret() (string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "AI-AAS Platform",
		AccountName: "user", // Will be replaced with actual user email during enrollment
		Algorithm:   otp.AlgorithmSHA1,
		Digits:      otp.DigitsSix,
		Period:      30,
	})
	if err != nil {
		return "", fmt.Errorf("generate TOTP secret: %w", err)
	}
	return key.Secret(), nil
}

// GenerateTOTPQRCode generates a QR code URL for TOTP enrollment.
// The URL can be used to generate a QR code that users can scan with authenticator apps.
func GenerateTOTPQRCode(secret, email, issuer string) string {
	if issuer == "" {
		issuer = "AI-AAS Platform"
	}
	// Return the otpauth:// URL that can be used to generate QR codes
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30",
		issuer, email, secret, issuer)
}

