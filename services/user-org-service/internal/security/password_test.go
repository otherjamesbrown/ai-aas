package security

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("Sup3rSecret!")
	require.NoError(t, err)
	require.NotEmpty(t, hash)

	ok, err := VerifyPassword("Sup3rSecret!", hash)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = VerifyPassword("WrongPassword", hash)
	require.NoError(t, err)
	require.False(t, ok)
}
