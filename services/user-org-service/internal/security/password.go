package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    uint32 = 1
	argonMemory  uint32 = 64 * 1024
	argonThreads uint8  = 4
	argonKeyLen  uint32 = 32
	saltLen             = 16
)

// HashPassword derives an Argon2id hash from the provided plaintext password.
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}

	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	encoded := fmt.Sprintf("argon2id$v=19$t=%d$m=%d$p=%d$%s$%s",
		argonTime,
		argonMemory,
		argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	return encoded, nil
}

// VerifyPassword compares a plaintext password with a stored Argon2id hash.
func VerifyPassword(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 7 {
		return false, errors.New("parse argon hash: unexpected format")
	}
	if parts[0] != "argon2id" {
		return false, errors.New("parse argon hash: invalid algorithm")
	}
	version, err := strconv.Atoi(strings.TrimPrefix(parts[1], "v="))
	if err != nil {
		return false, fmt.Errorf("parse argon hash version: %w", err)
	}
	if version != 19 {
		return false, fmt.Errorf("parse argon hash: unsupported version %d", version)
	}
	timeCost64, err := strconv.ParseUint(strings.TrimPrefix(parts[2], "t="), 10, 32)
	if err != nil {
		return false, fmt.Errorf("parse argon hash time: %w", err)
	}
	memCost64, err := strconv.ParseUint(strings.TrimPrefix(parts[3], "m="), 10, 32)
	if err != nil {
		return false, fmt.Errorf("parse argon hash memory: %w", err)
	}
	threadCost64, err := strconv.ParseUint(strings.TrimPrefix(parts[4], "p="), 10, 8)
	if err != nil {
		return false, fmt.Errorf("parse argon hash threads: %w", err)
	}
	saltB64 := parts[5]
	hashB64 := parts[6]

	salt, err := base64.RawStdEncoding.DecodeString(saltB64)
	if err != nil {
		return false, fmt.Errorf("decode salt: %w", err)
	}
	expectedHash, err := base64.RawStdEncoding.DecodeString(hashB64)
	if err != nil {
		return false, fmt.Errorf("decode hash: %w", err)
	}

	actualHash := argon2.IDKey(
		[]byte(password),
		salt,
		uint32(timeCost64),
		uint32(memCost64),
		uint8(threadCost64),
		uint32(len(expectedHash)),
	)
	if subtle.ConstantTimeCompare(actualHash, expectedHash) == 1 {
		return true, nil
	}
	return false, nil
}
