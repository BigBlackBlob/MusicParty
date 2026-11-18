# Bandcamp Subsonic compatibility probe

Run this standalone probe locally or in a test environment. Bandcamp introduced Subsonic support in beta in 2026. Its behavior may change.

Generate Subsonic credentials in Bandcamp under **Fan Settings → Subsonic**. From the MusicParty repo root, run the probe with those credentials.

```powershell
$env:BANDCAMP_USERNAME = 'your-subsonic-username'
$env:BANDCAMP_PASSWORD = 'your-subsonic-password'
go -C tools/bandcamp-probe run .
```

On Bash or another POSIX shell:

```sh
BANDCAMP_USERNAME=... BANDCAMP_PASSWORD=... go -C tools/bandcamp-probe run .
```

The default base URL is `https://bandcamp.com/api/subsonic`. Set `BANDCAMP_BASE_URL` to test a different endpoint.

## Read the result

- `COMPATIBLE` means no check failed. If search, stream and seek passed, add Bandcamp as a Subsonic source in the MusicParty admin UI.
- `NOT COMPATIBLE` lists the checks that failed and exits with code 1. Fall back to an embed or external link.
- `WARN` and `SKIP` do not count as failures. If `jazz` returns no songs, the probe cannot verify cover art, streaming or seeking. Repeat with a collection that returns a song for `jazz` before relying on the verdict.

The probe covers the six checks in the summary. MusicParty also uses `getSong` to resolve tracks and `getAlbum` to list album tracks.
