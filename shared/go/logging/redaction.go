package logging

import (
	"regexp"
	"strings"
)

// Redaction patterns for sensitive data in logs.
// These patterns are based on configs/log-redaction.yaml.
var (
	// PasswordPattern matches passwords in various formats.
	PasswordPattern = regexp.MustCompile(`(?i)(password[=:]\s*)([^\s"',}]+)`)

	// TokenPattern matches API tokens and bearer tokens.
	TokenPattern = regexp.MustCompile(`(?i)(Bearer\s+)([A-Za-z0-9\-_.]{20,})`)

	// ConnectionStringPattern matches database connection strings with credentials.
	ConnectionStringPattern = regexp.MustCompile(`://[^:]+:[^@]+@`)

	// APIKeyPattern matches API keys in headers or environment variables.
	APIKeyPattern = regexp.MustCompile(`(?i)(X-API-Key:\s*|Authorization:\s*Bearer\s+)([A-Za-z0-9\-_]{20,})`)

	// SecretPattern matches generic secrets in environment variables.
	SecretPattern = regexp.MustCompile(`(?i)([A-Z_]+SECRET[=:]\s*|[A-Z_]+_SECRET[=:]\s*)([^\s"',}]+)`)

	// TokenEnvPattern matches tokens in environment variables.
	TokenEnvPattern = regexp.MustCompile(`(?i)([A-Z_]+TOKEN[=:]\s*|[A-Z_]+_TOKEN[=:]\s*)([A-Za-z0-9\-_]{20,})`)
)

// RedactString applies redaction patterns to a string, masking sensitive data.
func RedactString(s string) string {
	if s == "" {
		return s
	}

	result := s

	// Redact passwords
	result = PasswordPattern.ReplaceAllString(result, `${1}***REDACTED***`)

	// Redact tokens
	result = TokenPattern.ReplaceAllString(result, `${1}***REDACTED***`)
	result = TokenEnvPattern.ReplaceAllString(result, `${1}***REDACTED***`)

	// Redact API keys
	result = APIKeyPattern.ReplaceAllString(result, `${1}***REDACTED***`)

	// Redact connection strings
	result = ConnectionStringPattern.ReplaceAllString(result, `://***REDACTED***@`)

	// Redact secrets
	result = SecretPattern.ReplaceAllString(result, `${1}***REDACTED***`)

	return result
}

// RedactFields redacts sensitive values in a map of fields.
func RedactFields(fields map[string]interface{}) map[string]interface{} {
	if fields == nil {
		return fields
	}

	redacted := make(map[string]interface{}, len(fields))
	sensitiveKeys := []string{"password", "secret", "token", "key", "credential", "auth"}

	for k, v := range fields {
		keyLower := strings.ToLower(k)
		isSensitive := false

		for _, sensitive := range sensitiveKeys {
			if strings.Contains(keyLower, sensitive) {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			if str, ok := v.(string); ok {
				redacted[k] = RedactString(str)
			} else {
				redacted[k] = "***REDACTED***"
			}
		} else {
			redacted[k] = v
		}
	}

	return redacted
}

// RedactValue redacts a single value if it appears to contain sensitive data.
func RedactValue(v interface{}) interface{} {
	if v == nil {
		return v
	}

	if str, ok := v.(string); ok {
		return RedactString(str)
	}

	return v
}

