# Go deployment utilities

This directory contains local operator tools only. Nothing here connects to a VPS or changes a running service automatically.

Production deployments must set `MUSIC_PARTY_IMAGE` to an immutable GHCR or ACR digest. The default container identity is `10001:10001`; an existing host directory may override `MUSIC_PARTY_RUNTIME_UID` and `MUSIC_PARTY_RUNTIME_GID` after its ownership has been verified.

## Preflight

```sh
export MUSIC_PARTY_IMAGE='ghcr.io/bigblackblob/musicparty@sha256:...'
docker pull "$MUSIC_PARTY_IMAGE"
sh backend-go/deploy/cutover.sh preflight
```

Preflight checks the immutable image reference, Compose configuration, database presence, image availability and configured UID/GID write access.

## Snapshot and verification

Enter an approved maintenance window and stop the only SQLite writer before creating a snapshot:

```sh
docker compose stop music-party
export MUSICPARTY_MAINTENANCE_CONFIRMED=YES
sh backend-go/deploy/cutover.sh snapshot
sh backend-go/deploy/cutover.sh verify ./music_party/backups/musicparty-<timestamp>.db
```

The tool uses the image's `/app/dbsnapshot` and `/app/dbcheck` binaries and prints the snapshot SHA-256. Existing snapshots are never overwritten.

## Local rehearsal

`prepare-isolated.py` copies a running deployment into an isolation directory and writes the runtime contract for a rehearsal container started from the same image:

```sh
python backend-go/deploy/prepare-isolated.py /opt/musicparty-rehearsal-<date>
python backend-go/deploy/prepare-isolated.py --verify-runtime-env /opt/musicparty-rehearsal-<date>
```

Rehearsals run with `APP_ENV=production`, so the binary refuses to start unless `ROOM_ACCESS_TOKEN_SECRET` is present. The script therefore provisions it into `runtime.env` and never copies the source container's key, because a rehearsal must not mint room access proofs the source deployment accepts. Resolution order is: the value exported by the operator, then the value already persisted in that isolation directory, then a generated 32-byte `secrets.token_urlsafe` value. The key is written at mode `0600`, chowned to the runtime uid/gid, and `rehearsal.json` only records its origin (`provided`/`persisted`/`generated`) and length. Rerunning the script against an existing isolation directory only refreshes `runtime.env`; it never re-copies data or rotates a persisted key.

```sh
python -m unittest discover -s backend-go/deploy -p 'test_*.py'
```

## Start and rollback

Start with the base Compose file only:

```sh
docker compose up -d --no-deps music-party
curl --fail --silent --show-error http://127.0.0.1:8848/actuator/health/readiness
```

Rollback means stopping the current Go writer, preserving failure evidence, restoring a verified pre-migration database snapshot when required, and setting `MUSIC_PARTY_IMAGE` to the previous known-good Go digest. Java images are historical artifacts and are not a supported rollback target.
