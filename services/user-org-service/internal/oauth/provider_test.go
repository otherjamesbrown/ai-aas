package oauth

import (
	"testing"

	"github.com/ory/fosite/storage"
	"github.com/stretchr/testify/require"
)

func TestNewProviderDefaults(t *testing.T) {
	memStore := storage.NewMemoryStore()
	secret := []byte("0123456789abcdef0123456789abcdef")

	provider, err := NewProvider(ProviderDependencies{
		Storage:    memStore,
		HMACSecret: secret,
	})
	require.NoError(t, err)
	require.NotNil(t, provider)
}

func TestNewProviderRequiresSecret(t *testing.T) {
	_, err := NewProvider(ProviderDependencies{
		Storage:    storage.NewMemoryStore(),
		HMACSecret: []byte("short"),
	})
	require.Error(t, err)
}

func TestNewProviderRequiresStorage(t *testing.T) {
	_, err := NewProvider(ProviderDependencies{
		HMACSecret: []byte("0123456789abcdef0123456789abcdef"),
	})
	require.Error(t, err)
}
