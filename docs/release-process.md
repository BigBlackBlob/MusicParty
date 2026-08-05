# Go image release process

## Default branch publication

An application-code push to `NRT-Base` runs the `CI` workflow. Contracts, Go checks, race tests, frontend checks, browser E2E, image scanning and container readiness must all pass before publication.

The same verified image is published as:

```text
ghcr.io/bigblackblob/musicparty:sha-<40-character commit SHA>
ghcr.io/bigblackblob/musicparty:nrt-base
crpi-533x5q1t88ew0x21.cn-hangzhou.personal.cr.aliyuncs.com/nrt-base/nrt-music-party:sha-<40-character commit SHA>
crpi-533x5q1t88ew0x21.cn-hangzhou.personal.cr.aliyuncs.com/nrt-base/nrt-music-party:nrt-base
```

Until `v1.1.0`, the workflow also updates the `ghcr.io/bigblackblob/musicparty-go` compatibility alias. A PR builds and scans but never logs in or pushes.

The Actions summary prints both immutable digest references and the source SHA. It never updates a VPS.

## Version tags

- `vX.Y.Z-rc.N` copies an existing verified SHA image to the exact tag and `rc`.
- `vX.Y.Z` copies it to the exact tag, `vX.Y`, `vX` and `latest`.
- Releases never rebuild an image.
- The tagged commit must be reachable from `NRT-Base`, have a successful `CI` run and already exist with the same revision and digest in both registries.

## Deploy and rollback

Copy an immutable reference from the Actions summary into the host's untracked environment file:

```text
MUSIC_PARTY_IMAGE=ghcr.io/bigblackblob/musicparty@sha256:...
```

Then run the host's reviewed Compose procedure. Production documentation must not pin `nrt-base` or `latest` because they are moving tags.

Rollback by restoring the previous known-good Go digest. If a schema migration was involved, stop the writer and restore its verified pre-migration snapshot before starting the previous image.
