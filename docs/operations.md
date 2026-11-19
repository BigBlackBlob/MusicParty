# MusicParty operations

This runbook covers the supported Go-only Docker deployment. Production must use an immutable image digest, never a moving tag.

## Deploy

Create an untracked `.env` with at least:

```dotenv
MUSIC_PARTY_IMAGE=ghcr.io/bigblackblob/musicparty@sha256:...
NETEASE_API_IMAGE=moefurina/ncm-api:4.41.1@sha256:2fcf4dc49313109a91f6cec99c6b36ebd17e8f1fdc89686121dfd14525937896
BASE_URL=https://music.example.com
ALLOWED_ORIGINS=https://music.example.com
BOOTSTRAP_ADMIN_USERNAME=admin
BOOTSTRAP_ADMIN_PASSWORD=replace-me
MUSIC_PARTY_RUNTIME_UID=10001
MUSIC_PARTY_RUNTIME_GID=10001
```

Ensure the bind-mounted `music_party/data` and `music_party/cached_media` directories are writable by the configured runtime UID/GID. Existing installations may retain their established UID/GID explicitly.

```bash
docker compose config --quiet
docker compose pull
docker compose up -d
curl --fail http://127.0.0.1:8848/actuator/health/readiness
```

Platform credentials should be saved through the administrator settings or injected through untracked environment configuration during migration. Never print them in support logs.

## Spotify playback experiment

1. Register a Spotify app with Web API and Web Playback SDK access. Set its redirect URI to your exact public HTTPS origin followed by `/spotify/callback`. Add the listening accounts to the app's allowlist when running in Development Mode.
2. Set `SPOTIFY_ENABLED=true`, `SPOTIFY_CLIENT_ID`, and `SPOTIFY_CLIENT_SECRET` in the server environment, then restart MusicParty. `SPOTIFY_MARKET` defaults to `US` and selects the catalog used for search and queue resolution.
3. Queue a Spotify track and select **Connect Spotify**, then **Enable Spotify audio**. Each listener uses their own Premium account. Reloading the app requires reconnecting.

Log only the path for `/spotify/callback` at the reverse proxy so the query does not record OAuth codes. If the proxy sets CSP, allow `https://sdk.scdn.co` for scripts and `https://accounts.spotify.com` and `https://api.spotify.com` for connections. Check the browser's report-only CSP output for SDK-managed requests before enforcing the policy.

## Observe

```bash
docker compose ps
docker compose logs --tail=200 music-party
curl --fail http://127.0.0.1:8848/actuator/health
curl --fail http://127.0.0.1:8848/actuator/health/readiness
curl --fail http://127.0.0.1:8848/actuator/prometheus
```

Readiness proves the process can serve traffic; it does not replace functional checks for enabled music platforms.

## Backup

Use the bundled `dbsnapshot` tool or SQLite's online backup mechanism while the service is running. Record the image digest and snapshot SHA-256 together. Verify the snapshot with `dbcheck`, `PRAGMA integrity_check` and `PRAGMA foreign_key_check`, and store it outside the application data directory.

## Upgrade

1. Read the successful `CI` summary and copy the GHCR or ACR immutable digest.
2. Create and verify a database snapshot before any schema-changing release.
3. Update only `MUSIC_PARTY_IMAGE` in the deployment environment.
4. Pull and recreate the application container.
5. Verify readiness, login, room connection, presence, queue, chat and enabled platforms.

GitHub Actions does not update the VPS.

## Rollback

Stop the current Go writer. If the release migrated the schema, restore its paired pre-migration snapshot. Set `MUSIC_PARTY_IMAGE` to the previous known-good Go digest, recreate the container and run the same verification. Historical Java images are not supported rollback targets.

## Incident safety

- Preserve logs and a database snapshot before repair attempts.
- Do not run tests or contract generators against production data.
- Distinguish session rejection from room-access rejection and platform credential failures.
- Rotate any external credential that has entered Git, logs, screenshots or support transcripts.
- Avoid recursive permission changes outside the two explicit bind-mount directories.
