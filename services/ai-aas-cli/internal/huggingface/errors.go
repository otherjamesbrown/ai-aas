// Package huggingface re-exports the shared huggingface errors for backward compatibility.
package huggingface

import sharedHF "github.com/otherjamesbrown/ai-aas/shared/go/modelcache/huggingface"

// Re-export errors
var (
	ErrModelNotFound      = sharedHF.ErrModelNotFound
	ErrRevisionNotFound   = sharedHF.ErrRevisionNotFound
	ErrUnauthorized       = sharedHF.ErrUnauthorized
	ErrForbidden          = sharedHF.ErrForbidden
	ErrNoToken            = sharedHF.ErrNoToken
	ErrGatedModel         = sharedHF.ErrGatedModel
	ErrLicenseNotAccepted = sharedHF.ErrLicenseNotAccepted
	ErrDownloadFailed     = sharedHF.ErrDownloadFailed
	ErrChecksumMismatch   = sharedHF.ErrChecksumMismatch
)
