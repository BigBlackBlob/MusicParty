package httpapi

import (
	"bytes"
	"testing"
	"time"
)

// legacyPublicRoomAccessSecret is the constant this package used to fall back
// to when ROOM_ACCESS_TOKEN_SECRET was empty. It shipped in the source tree, so
// anyone could read it; proofs signed with it must never validate.
const legacyPublicRoomAccessSecret = "musicparty-room-access-development-secret"

func TestRoomAccessSecretFallsBackToProcessEphemeralKey(t *testing.T) {
	first := roomAccessSecret("   ")
	second := roomAccessSecret("")
	if !bytes.Equal(first, second) {
		t.Fatal("empty configuration produced different secrets within one process")
	}
	if bytes.Equal(first, []byte(legacyPublicRoomAccessSecret)) {
		t.Fatal("empty configuration fell back to the public source constant")
	}
	if len(first) == 0 {
		t.Fatal("empty configuration produced an empty secret")
	}
}

func TestRoomAccessSecretUsesConfiguredValue(t *testing.T) {
	configured := "configured-room-access-secret"
	if got := string(roomAccessSecret(configured)); got != configured {
		t.Fatalf("roomAccessSecret() = %q, want configured value", got)
	}
	if got := string(roomAccessSecret("  " + configured + "  ")); got != configured {
		t.Fatalf("roomAccessSecret() = %q, want trimmed configured value", got)
	}
}

func TestValidRoomAccessTokenRejectsPublicConstantProof(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(time.Minute).UnixMilli()
	forged := signRoomAccessToken([]byte(legacyPublicRoomAccessSecret), "room-1", "public-1", expiresAt, 1)

	secret := roomAccessSecret("")
	if validRoomAccessToken(secret, forged, "room-1", "public-1", 1, now) {
		t.Fatal("proof signed with the public constant was accepted")
	}

	genuine := signRoomAccessToken(secret, "room-1", "public-1", expiresAt, 1)
	if !validRoomAccessToken(secret, genuine, "room-1", "public-1", 1, now) {
		t.Fatal("proof signed with the process secret was rejected")
	}
}
