package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecryptSubsonicCredentialMatchesJavaFormat(t *testing.T) {
	secret := "site-secret"
	plain := "pässword"
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)
	nonce := []byte("123456789012")
	payload := append(append([]byte{}, nonce...), gcm.Seal(nil, nonce, []byte(plain), nil)...)
	stored := encryptedCredentialPrefix + base64.StdEncoding.EncodeToString(payload)
	got, err := DecryptSubsonicCredential(stored, secret)
	require.NoError(t, err)
	require.Equal(t, plain, got)
}
