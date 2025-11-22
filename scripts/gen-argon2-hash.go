package main

import (
	"fmt"
	"golang.org/x/crypto/argon2"
	"crypto/rand"
	"encoding/base64"
)

func main() {
	password := "e2e-admin-password"

	// Generate salt
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		panic(err)
	}

	// Argon2id parameters (standard defaults)
	time := uint32(1)
	memory := uint32(64 * 1024) // 64MB
	threads := uint8(4)
	keyLen := uint32(32)

	// Generate hash
	hash := argon2.IDKey([]byte(password), salt, time, memory, threads, keyLen)

	// Format as $argon2id$v=19$m=65536,t=1,p=4$<base64-salt>$<base64-hash>
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encoded := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		memory, time, threads, b64Salt, b64Hash)

	fmt.Println(encoded)
}
