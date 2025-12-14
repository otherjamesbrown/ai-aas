// Package huggingface re-exports the shared license functionality for backward compatibility.
package huggingface

import sharedHF "github.com/otherjamesbrown/ai-aas/shared/go/modelcache/huggingface"

// Re-export license types
type LicenseInfo = sharedHF.LicenseInfo

// Re-export license functions and variables
var (
	CommonLicenses       = sharedHF.CommonLicenses
	GetLicenseFullName   = sharedHF.GetLicenseFullName
	IsRestrictiveLicense = sharedHF.IsRestrictiveLicense
)
