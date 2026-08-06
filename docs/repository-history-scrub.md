# Repository history scrub runbook

This is a destructive maintenance operation. Do not begin it until all external platform credentials formerly stored in runtime databases have been rotated and functionally verified on the VPS.

## Scope

Rewrite only `NRT-Base` and the annotated tags `go-rewrite-baseline-v1` and `legacy-java-master-20260802` to remove these paths from every reachable commit:

```text
music_party/data/musicparty.db
music_party/cached_media/BV1JR4y117KH.m4a
backend-go/bin/musicparty.exe
```

Preserve tag names and messages. Existing signatures cannot be preserved. Published images and the running VPS are out of scope.

## Preconditions

- Announce a push freeze and confirm no release workflow is running.
- Record the old branch SHA and annotated tag-object SHAs.
- Create a repository-external Git bundle, restrict its permissions and run `git bundle verify`.
- Create temporary remote `refs/cleanup-backup/*` refs for the branch and both tags.
- Use a clean mirror clone and a pinned `git-filter-repo` version.

## Verification

- `git rev-list --all --objects` contains none of the three paths.
- Scan reachable objects for reusable cookie values, bearer tokens, `.env` secrets, SQLite files, media and executables.
- Confirm the current tree still contains the Go backend, frontend, frozen schema, three workflows, root Dockerfile, Compose and current documentation.
- Run `git fsck --full` and inspect `git count-objects -vH`.
- Clone the rewritten repository afresh and repeat the object/path checks.

## Publication

Push the branch and both tags atomically with explicit `--force-with-lease` expectations based on the recorded old refs. Abort on any mismatch; never fall back to unconditional force.

Delete temporary remote backup refs only after GitHub shows the rewritten refs, a fresh clone verifies, the new `NRT-Base` CI run has started successfully, and the external bundle remains available for short-term recovery. Do not deploy the resulting image automatically.
