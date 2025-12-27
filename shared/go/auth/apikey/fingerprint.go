// Package apikey provides API key utilities for ai-aas services.
//
// This package consolidates API key handling including fingerprint computation,
// header extraction, and authentication context types.
package apikey

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// FingerprintFormat defines the encoding format for fingerprints.
type FingerprintFormat int

const (
	// FingerprintHex uses hexadecimal encoding.
	// This format is used by api-router-service.
	FingerprintHex FingerprintFormat = iota

	// FingerprintBase64URL uses base64 URL-safe encoding without padding.
	// This format is used by user-org-service for database storage.
	FingerprintBase64URL
)

// ComputeFingerprint computes the SHA-256 fingerprint of an API key.
// The format parameter determines the output encoding.
func ComputeFingerprint(apiKey string, format FingerprintFormat) string {
	hash := sha256.Sum256([]byte(apiKey))

	switch format {
	case FingerprintBase64URL:
		return base64.RawURLEncoding.EncodeToString(hash[:])
	case FingerprintHex:
		fallthrough
	default:
		return hex.EncodeToString(hash[:])
	}
}

// ComputeFingerprintHex computes a hex-encoded SHA-256 fingerprint.
// This is a convenience function for the common hex encoding case.
func ComputeFingerprintHex(apiKey string) string {
	return ComputeFingerprint(apiKey, FingerprintHex)
}

// ComputeFingerprintBase64URL computes a base64url-encoded SHA-256 fingerprint.
// This is a convenience function for the base64 URL encoding case.
func ComputeFingerprintBase64URL(apiKey string) string {
	return ComputeFingerprint(apiKey, FingerprintBase64URL)
}
