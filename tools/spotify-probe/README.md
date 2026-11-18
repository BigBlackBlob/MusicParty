# Spotify account-suitability probe

Checks whether your Spotify Premium account is compatible with go-librespot audio decryption. Run this before any Spotify integration work. Proceed only if the probe reports `COMPATIBLE`.

---

## Prerequisites

1. A Spotify Premium account.
2. A Spotify app registered at [developer.spotify.com](https://developer.spotify.com/dashboard):
   - Add redirect URI: `http://127.0.0.1:3678/callback`
   - Enable scopes: `streaming`, `user-read-email`, `user-read-private`
   - Copy the **Client ID** (you need it as `SPOTIFY_CLIENT_ID`)
3. Go 1.21+ installed on both your Windows dev machine and the Linux server.
4. On the Linux server: `libvorbis-dev`, `libflac-dev`, and `libmp3lame-dev` (for the audio decoder CGO dependencies).

---

## Step 1 — OAuth login (run on Windows)

```powershell
$env:SPOTIFY_CLIENT_ID = "your-client-id-here"
go run . login --engine go --credential-dir "$env:LOCALAPPDATA\MusicParty\spotify-probe"
```

This opens your browser for Spotify consent. After you approve, `credentials.json` is saved to the credential dir. **Copy that file to your Linux server** before running the check.

---

## Step 2 — Audio compatibility check (run on Linux server)

```bash
CRED_DIR="$HOME/.local/share/musicparty/spotify-probe"
mkdir -p "$CRED_DIR"
# Copy credentials.json here from Windows first, then:
go run . check --engine go \
  --credential-dir "$CRED_DIR" \
  --track "spotify:track:4uLU6hMCjMI75M1A2tKUQC"
```

Use any full-length track that plays in the official Spotify client on your account. The bare track ID (22 characters) also works instead of the full URI.

---

## Exit codes

| Code | Meaning |
|------|---------|
| `0` — `COMPATIBLE` | At least 5 s of valid audio decoded. Integration may proceed. |
| `1` — `NOT COMPATIBLE` | Audio key was explicitly denied by Spotify. The account cannot be used with this integration path. |
| `2` — `INCONCLUSIVE` | Network failure, timeout, invalid track, or other non-key failure. Fix the issue and rerun. |

---

## Rotating credentials

Re-run `login` to replace `credentials.json` with fresh tokens. The check step also rotates credentials automatically if go-librespot returns updated stored credentials after session establishment.

---

## Security notes

- `credentials.json` contains a reusable Spotify session credential. Keep it out of git (already in `.gitignore`).
- To revoke access without changing your password: go to [spotify.com/account/apps](https://www.spotify.com/account/apps) and remove the app.
- The probe never logs your credentials, tokens, or session data. Secrets are redacted from all error messages.
