# Spotify account-suitability probe

Checks whether your Spotify Premium account is compatible with go-librespot audio decryption. Run this before any Spotify integration work. Proceed only if the probe reports `COMPATIBLE`.

The probe uses go-librespot's built-in device authorization flow. No Spotify developer app registration, no `SPOTIFY_CLIENT_ID`, and no browser on the server are required — you approve a pairing code on any device you are already signed in to.

---

## Prerequisites

1. A Spotify Premium account.
2. Go 1.21+ on a Linux server. Audio decode requires CGO, so `check` runs only on Linux; there is no Windows build of the check step.
3. Audio decoder development libraries. On Rocky / RHEL / Fedora:
   ```bash
   sudo dnf install -y lame-devel libvorbis-devel flac-devel mpg123-devel alsa-lib-devel
   ```
   On Debian / Ubuntu:
   ```bash
   sudo apt install -y libmp3lame-dev libvorbis-dev libflac-dev libmpg123-dev libasound2-dev
   ```

---

## Step 1 — Device login (run on the Linux server)

```bash
CRED_DIR="$HOME/.local/share/musicparty/spotify-probe"
mkdir -p "$CRED_DIR"
go run . login --engine go --credential-dir "$CRED_DIR"
```

The command prints a `https://spotify.com/pair?code=XXXX` URL and a user code. Open the URL on any device, sign in with the Spotify Premium account, and approve. The pairing code expires after about 5 minutes, so approve promptly. On success, `credentials.json` is written to the credential dir.

---

## Step 2 — Audio compatibility check (run on the same Linux server)

```bash
go run . check --engine go \
  --credential-dir "$CRED_DIR" \
  --track "spotify:track:4uLU6hMCjMI75M1A2tKUQC"
```

Use any full-length track that plays in the official Spotify client on your account. The bare track ID (22 characters) also works instead of the full URI.

The check opens a librespot session, requests the audio key for the track, decodes at least 5 seconds of PCM, and verifies the samples are non-silent. It also writes back rotated stored credentials if go-librespot returns updated data after session establishment.

---

## Exit codes

| Code | Meaning |
|------|---------|
| `0` — `COMPATIBLE` | At least 5 s of valid audio decoded. Integration may proceed. |
| `1` — `NOT COMPATIBLE` | Audio key was explicitly denied by Spotify (`failed retrieving aes key`). The account cannot be used with this integration path. |
| `2` — `INCONCLUSIVE` | Network failure, timeout, invalid track, or other non-key failure. Fix the issue and rerun. |

---

## Rotating credentials

Re-run `login` to replace `credentials.json` with a fresh pairing. Spotify may revoke stored credentials after a password change or when the app is removed from your account; re-run `login` in that case.

---

## Security notes

- `credentials.json` contains a reusable Spotify session credential. Keep it out of git (already in `.gitignore`).
- The probe uses go-librespot's own Spotify client ID for the device flow; no third-party developer app is registered.
- To revoke access: change your Spotify password, or sign out of all devices in your Spotify account settings.
- The probe never logs your credentials, tokens, or session data. Secrets are redacted from all error messages.
