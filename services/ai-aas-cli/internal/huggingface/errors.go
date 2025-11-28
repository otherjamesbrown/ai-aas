// Package huggingface provides a client for interacting with the HuggingFace Hub API.
package huggingface

import "errors"

// Common errors
var (
	// ErrModelNotFound indicates the model was not found on HuggingFace Hub
	ErrModelNotFound = errors.New("model not found on HuggingFace Hub")

	// ErrRevisionNotFound indicates the specified revision was not found
	ErrRevisionNotFound = errors.New("revision not found")

	// ErrUnauthorized indicates invalid or missing authentication
	ErrUnauthorized = errors.New("unauthorized: invalid or missing token")

	// ErrForbidden indicates access is forbidden (e.g., gated model without access)
	ErrForbidden = errors.New("forbidden: access denied to this model")

	// ErrNoToken indicates no token was provided for an operation that requires one
	ErrNoToken = errors.New("no HuggingFace token configured")

	// ErrGatedModel indicates the model requires license acceptance
	ErrGatedModel = errors.New("model is gated and requires license acceptance")

	// ErrLicenseNotAccepted indicates the user hasn't accepted the model's license
	ErrLicenseNotAccepted = errors.New("license not accepted for this model")

	// ErrDownloadFailed indicates a download operation failed
	ErrDownloadFailed = errors.New("download failed")

	// ErrChecksumMismatch indicates a file checksum doesn't match expected value
	ErrChecksumMismatch = errors.New("checksum mismatch")
)

