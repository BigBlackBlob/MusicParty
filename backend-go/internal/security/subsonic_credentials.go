package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

const encryptedCredentialPrefix = "enc:v1:"

func DecryptSubsonicCredential(stored, siteSecret string) (string, error) {
	if !strings.HasPrefix(stored, encryptedCredentialPrefix) {
		return stored, nil
	}
	if strings.TrimSpace(siteSecret) == "" {
		return "", errors.New("site secret is unavailable")
	}
	payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, encryptedCredentialPrefix))
	if err != nil {
		return "", err
	}
	if len(payload) < 12+16 {
		return "", errors.New("encrypted credential payload is too short")
	}
	key := sha256.Sum256([]byte(siteSecret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	returnValue, err := gcm.Open(nil, payload[:12], payload[12:], nil)
	if err != nil {
		return "", err
	}
	return string(returnValue), nil
}
func EncryptSubsonicCredential(plain, siteSecret string) (string, error) {
	if strings.TrimSpace(plain) == "" || strings.HasPrefix(plain, encryptedCredentialPrefix) {
		return plain, nil
	}
	if strings.TrimSpace(siteSecret) == "" {
		return "", errors.New("site secret is unavailable")
	}
	key := sha256.Sum256([]byte(siteSecret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	payload := append(append([]byte{}, nonce...), gcm.Seal(nil, nonce, []byte(plain), nil)...)
	return encryptedCredentialPrefix + base64.StdEncoding.EncodeToString(payload), nil
}
