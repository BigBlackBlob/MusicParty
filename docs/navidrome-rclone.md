# Navidrome + Rclone optional deployment

This module adds a private Navidrome library to MusicParty without changing the default deployment. Navidrome is only used by the web and mobile browser player. The MusicParty HTTP radio stream is not adapted for Navidrome in this version.

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

## Permissions

`NAVIDROME_ALLOWED_USERS` is a lightweight username whitelist, not strong authentication. It is suitable for trusted rooms, but it is not recommended for exposing a private library in a fully public room.

Use names that are not easy to guess. Whitelisted users should enter the room first and occupy their username before opening the room to others.

Rules:

- Only non-guest MusicParty users can use Navidrome.
- Usernames are trimmed and matched case-insensitively.
- Empty `NAVIDROME_ALLOWED_USERS` means no regular user can use Navidrome, even when `NAVIDROME_ENABLED=true`.
- The admin password does not grant Navidrome access.

## Access boundaries

This integration is intentionally lightweight:

- MusicParty usernames are used as a trusted-room whitelist. They are not a strong identity or permission system.
- Navidrome stream proxy requests require the MusicParty user token.
- Navidrome cover proxy requests are not token-checked in this version. They do not expose Subsonic credentials, but cover art should not be treated as strongly private.
- The MusicParty HTTP radio stream does not support Navidrome tracks in this version. Browser playback is the supported playback path.

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
curl "http://127.0.0.1:8848/api/platforms?token=<your-token>"
```

Authorized users should see `navidrome` in the platform list. Unauthorized users and guests should not.

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

The script reads `.env.local` automatically, points Navidrome to `http://127.0.0.1:4533`, and runs a Subsonic `ping.view` precheck before starting the backend. Use `--env-file <path>` when you want a different local profile.

In this version, stream proxy requests require the MusicParty user token, while cover proxy requests are intentionally not token-checked. This is part of the lightweight trusted-room model and should not be treated as strong access control.
