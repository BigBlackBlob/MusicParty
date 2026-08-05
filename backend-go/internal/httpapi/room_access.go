package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

func roomAccessSecret(raw string) []byte {
	if strings.TrimSpace(raw) != "" {
		return []byte(raw)
	}
	// Development configurations may omit the explicit secret. This still keeps
	// access proofs process-local and avoids exposing a browser-readable token.
	return []byte("musicparty-room-access-development-secret")
}

func signRoomAccessToken(secret []byte, roomID, publicID string, expiresAt int64, passwordVersion int) string {
	payload := strings.Join([]string{roomID, publicID, strconv.FormatInt(expiresAt, 10), strconv.Itoa(passwordVersion)}, "|")
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func validRoomAccessToken(secret []byte, token, roomID, publicID string, passwordVersion int, now time.Time) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || publicID == "" {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return false
	}
	fields := strings.Split(string(payload), "|")
	if len(fields) != 4 {
		return false
	}
	expiresAt, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return false
	}
	version, err := strconv.Atoi(fields[3])
	return err == nil && fields[0] == roomID && fields[1] == publicID && expiresAt >= now.UnixMilli() && version == passwordVersion
}
