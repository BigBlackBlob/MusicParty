package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
)

// ephemeralRoomAccessSecret backs room access proofs when
// ROOM_ACCESS_TOKEN_SECRET is not configured. It is generated once per process
// so that a value baked into the source tree cannot be used to forge proofs,
// and so tokens issued before a restart stop validating afterwards.
var (
	ephemeralRoomAccessSecretOnce sync.Once
	ephemeralRoomAccessSecret     []byte
)

func newEphemeralRoomAccessSecret() []byte {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		// crypto/rand only fails when the platform RNG is unavailable; still
		// avoid shipping an all-zero key by mixing in per-process entropy that
		// is not present in the source tree.
		stamp := []byte(strconv.FormatInt(time.Now().UnixNano(), 10))
		for index := range secret {
			secret[index] = stamp[index%len(stamp)] ^ byte(index)
		}
	}
	return secret
}

func roomAccessSecret(raw string) []byte {
	if configured := strings.TrimSpace(raw); configured != "" {
		return []byte(configured)
	}
	ephemeralRoomAccessSecretOnce.Do(func() {
		ephemeralRoomAccessSecret = newEphemeralRoomAccessSecret()
	})
	return ephemeralRoomAccessSecret
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

func roomAccessAllowedRequest(r *http.Request, store *storesqlite.Store, secret []byte, roomID string, session account.Session) bool {
	proof, _ := r.Cookie(RoomAccessCookieName)
	return roomAccessAllowed(r.Context(), store, secret, roomID, session, cookieValue(proof))
}

func roomAccessAllowed(ctx context.Context, store *storesqlite.Store, secret []byte, roomID string, session account.Session, proof string) bool {
	var visibility, owner string
	var passwordVersion int
	if err := store.Reader().QueryRowContext(ctx, "select visibility,password_version,owner_public_id from room where id=? and deleted_at is null", roomID).Scan(&visibility, &passwordVersion, &owner); err != nil {
		return false
	}
	if visibility != "PRIVATE" || session.Admin() || (owner != "" && owner == session.PublicID) {
		return true
	}
	return validRoomAccessToken(secret, proof, roomID, session.PublicID, passwordVersion, time.Now())
}
