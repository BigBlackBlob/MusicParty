# Navidrome + Rclone optional deployment

This module adds a private Navidrome library to MusicParty without changing the default deployment. Navidrome is used by the web and mobile browser player.

## Start

Create `.env.navidrome` from `.env.navidrome.example`, then set the Navidrome account and MusicParty username whitelist.

```bash
docker compose -f docker-compose.yml -f docker-compose.navidrome.yml --env-file .env.navidrome up -d
```

## Update

```bash
docker compose pull music-party
docker compose -f docker-compose.yml -f docker-compose.navidrome.yml --env-file .env.navidrome up -d
```

`music-party` uses `MUSIC_PARTY_IMAGE` from `docker-compose.yml`. Keep the default for GHCR, or set it to the Aliyun ACR image before running the same commands:

```bash
MUSIC_PARTY_IMAGE=crpi-533x5q1t88ew0x21.cn-hangzhou.personal.cr.aliyuncs.com/nrt-base/nrt-music-party@sha256:<digest> docker compose -f docker-compose.yml -f docker-compose.navidrome.yml --env-file .env.navidrome up -d
```

## Permissions

`NAVIDROME_ALLOWED_USERS` is a compatibility allowlist for persistent MusicParty accounts. Platform-administrator configuration and room capabilities remain the authoritative management boundary.

Rules:

- Only non-guest MusicParty users can use Navidrome.
- Account names are trimmed and matched case-insensitively.
- Empty `NAVIDROME_ALLOWED_USERS` means no regular user can use Navidrome, even when `NAVIDROME_ENABLED=true`.
- The admin password does not grant Navidrome access.

## Access boundaries

This integration is intentionally lightweight:

- MusicParty persistent account names may be used as an additional allowlist; authentication still comes from the HttpOnly MusicParty session.
- Navidrome stream proxy requests require the MusicParty user token.
- Navidrome cover proxy requests are not token-checked in this version. They do not expose Subsonic credentials, but cover art should not be treated as strongly private.

## VPS checks

```bash
ls -l /dev/fuse
free -h
df -h
dd if=/dev/zero of=/tmp/musicparty-io-test bs=1M count=100 conv=fdatasync
rm -f /tmp/musicparty-io-test
```

## Verify

Open the room in a browser, set your MusicParty username to a whitelisted name, then check:

```bash
curl --cookie /path/to/non-production-session-cookie http://127.0.0.1:8848/api/platforms
```

Authorized members should see `navidrome` in the platform list. Unauthorized users and guests should not.

Navidrome audio URLs exposed to the browser should use MusicParty paths like `/api/navidrome/stream/...`; Navidrome `/rest/stream.view` URLs and Navidrome credentials should never be sent to the browser.

## Local development on Windows

For day-to-day debugging with the Windows-native Navidrome service, keep credentials in an ignored `.env.local` file:

```bash
cp .env.local.example .env.local
```

Edit `.env.local`, then start MusicParty from Git Bash:

```bash
./start-dev.sh --navidrome-local
```

The script reads `.env.local` automatically and points Navidrome to `http://127.0.0.1:4533`. Verify the source through the administrator settings after startup. Use `--env-file <path>` for a different local profile.

In this version, stream proxy requests require the MusicParty user token, while cover proxy requests are intentionally not token-checked. This is part of the lightweight trusted-room model and should not be treated as strong access control.
