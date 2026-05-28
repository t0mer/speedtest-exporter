package crypto_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/t0mer/speedtest-exporter/internal/crypto"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	plaintext := []byte("hello, notifications!")
	ct, err := crypto.Encrypt(key, plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ct)

	got, err := crypto.Decrypt(key, ct)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}

func TestEncryptProducesUniqueNonces(t *testing.T) {
	key := make([]byte, 32)
	ct1, err := crypto.Encrypt(key, []byte("same"))
	require.NoError(t, err)
	ct2, err := crypto.Encrypt(key, []byte("same"))
	require.NoError(t, err)
	assert.NotEqual(t, ct1, ct2, "each call must use a fresh nonce")
}

func TestDecryptWrongKey(t *testing.T) {
	key := make([]byte, 32)
	ct, err := crypto.Encrypt(key, []byte("secret"))
	require.NoError(t, err)
	wrong := make([]byte, 32)
	wrong[0] = 0xff
	_, err = crypto.Decrypt(wrong, ct)
	assert.Error(t, err)
}

func TestLoadOrCreateKey(t *testing.T) {
	dir := t.TempDir()
	key1, err := crypto.LoadOrCreateKey(dir)
	require.NoError(t, err)
	assert.Len(t, key1, 32)
	key2, err := crypto.LoadOrCreateKey(dir)
	require.NoError(t, err)
	assert.Equal(t, key1, key2)
	_, err = os.Stat(dir + "/.encryption_key")
	assert.NoError(t, err)
}
