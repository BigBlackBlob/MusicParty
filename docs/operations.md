# MusicParty operations

This runbook covers the supported Go-only Docker deployment. Production must use an immutable image digest, never a moving tag.

## Deploy

Create an untracked `.env` with at least:

```dotenv
MUSIC_PARTY_IMAGE=ghcr.io/bigblackblob/musicparty@sha256:...
NETEASE_API_IMAGE=binaryify/neteasecloudmusicapi:4.21.3
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
