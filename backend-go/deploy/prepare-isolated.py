"""Snapshot a running deployment for rehearsal without changing its container or data."""
import json
import os
import pathlib
import secrets
import shutil
import stat
import subprocess
import sys

ROOM_ACCESS_SECRET_KEY = "ROOM_ACCESS_TOKEN_SECRET"
# backend-go/internal/config/config.go refuses to boot a production deployment
# whose signing key is shorter than this, so a shorter generated value would only
# be discovered when the rehearsal container fails to start.
MIN_ROOM_ACCESS_SECRET_LENGTH = 16
GENERATED_ROOM_ACCESS_SECRET_BYTES = 32
RUNTIME_ENV_NAME = "runtime.env"
RECORD_NAME = "rehearsal.json"


def usage():
    raise SystemExit(
        "usage: prepare-isolated.py <isolation-directory>\n"
        "       prepare-isolated.py --verify-runtime-env <isolation-directory>"
    )


def read_room_access_secret(path):
    """Return the persisted signing key, or None when the file or the key is absent."""
    try:
        lines = path.read_text().splitlines()
    except FileNotFoundError:
        return None
    for line in lines:
        key, separator, value = line.partition("=")
        if separator and key == ROOM_ACCESS_SECRET_KEY:
            return value or None
    return None


def resolve_room_access_secret(environment, persisted_path):
    """Choose the room access signing key and report where it came from.

    Precedence is: the value the caller exported, then the value this script
    already persisted under the isolation root, then a freshly generated key.
    The key inherited from the source container is deliberately discarded so a
    rehearsal can never mint room access proofs the source deployment accepts.
    """
    environment.pop(ROOM_ACCESS_SECRET_KEY, None)
    provided = (os.environ.get(ROOM_ACCESS_SECRET_KEY) or "").strip()
    if provided:
        return provided, "provided"
    persisted = read_room_access_secret(persisted_path)
    if persisted:
        return persisted, "persisted"
    return secrets.token_urlsafe(GENERATED_ROOM_ACCESS_SECRET_BYTES), "generated"


def write_runtime_environment(root, environment):
    """Write runtime.env at mode 0600. The returned record never contains the key."""
    path = root / RUNTIME_ENV_NAME
    for key, value in environment.items():
        if "\n" in value or "\r" in value:
            raise RuntimeError("Multiline environment value needs explicit provisioning: " + key)
    secret = environment.get(ROOM_ACCESS_SECRET_KEY, "")
    if len(secret) < MIN_ROOM_ACCESS_SECRET_LENGTH:
        raise RuntimeError(
            ROOM_ACCESS_SECRET_KEY + " must be at least " + str(MIN_ROOM_ACCESS_SECRET_LENGTH) + " characters"
        )
    path.write_text("".join(key + "=" + value + "\n" for key, value in environment.items()))
    os.chmod(path, 0o600)
    return path


def require_owner_only(mode):
    """Reject any permission bits beyond the owner's read/write."""
    checked = stat.S_IMODE(mode)
    if checked != 0o600:
        raise RuntimeError("runtime.env must be readable only by its owner; found mode " + oct(checked))


def verify_runtime_environment(path):
    """Check a persisted runtime.env: owner-only mode, present, long enough, single line.

    Deliberately returns only a length so verification can be re-run without ever
    echoing the secret.
    """
    require_owner_only(os.stat(path).st_mode)
    secret = read_room_access_secret(path)
    if not secret:
        raise RuntimeError(ROOM_ACCESS_SECRET_KEY + " is missing from " + str(path))
    if "\n" in secret or "\r" in secret:
        raise RuntimeError(ROOM_ACCESS_SECRET_KEY + " must be a single line")
    if secret.strip() != secret:
        raise RuntimeError(ROOM_ACCESS_SECRET_KEY + " must not contain surrounding whitespace")
    if len(secret) < MIN_ROOM_ACCESS_SECRET_LENGTH:
        raise RuntimeError(
            ROOM_ACCESS_SECRET_KEY + " must be at least " + str(MIN_ROOM_ACCESS_SECRET_LENGTH) + " characters"
        )
    return len(secret)


def chown_tree(root, uid, gid):
    for directory, folders, files in os.walk(root):
        os.chown(directory, uid, gid)
        for file in files:
            os.chown(pathlib.Path(directory) / file, uid, gid)


def main():
    arguments = sys.argv[1:]
    if arguments[:1] == ["--verify-runtime-env"]:
        if len(arguments) != 2:
            usage()
        root = pathlib.Path(arguments[1]).resolve()
        length = verify_runtime_environment(root / RUNTIME_ENV_NAME)
        print(json.dumps({"runtimeEnvironment": str(root / RUNTIME_ENV_NAME), "roomAccessSecretLength": length}))
        return

    if len(arguments) != 1:
        usage()
    root = pathlib.Path(arguments[0]).resolve()
    if root.exists():
        # A rerun must never re-copy data or rotate the persisted signing key; it
        # only refreshes runtime.env from the current source container so a rehearsal
        # that was prepared before this key existed can be started again.
        record_path = root / RECORD_NAME
        if not (record_path.is_file() and (root / "data" / "musicparty.db").is_file()):
            raise RuntimeError("Isolation directory already exists")
        record = json.loads(record_path.read_text())
        environment = json.loads(
            subprocess.check_output(["docker", "inspect", "musicparty-docker-test-music-party-1"])
        )[0]["Config"]["Env"]
        environment = dict(entry.split("=", 1) for entry in environment)
        environment.update(
            {
                "BASE_URL": "http://music-party-rehearsal:8080",
                "ALLOWED_ORIGINS": "http://music-party-rehearsal:8080,http://127.0.0.1:18848",
                "APP_ENV": "production",
                "AUTH_SECURE_COOKIES": "false",
            }
        )
        for key in ("NODE_ENV", "ENV", "SPRING_PROFILES_ACTIVE"):
            environment.pop(key, None)
        secret, origin = resolve_room_access_secret(environment, root / RUNTIME_ENV_NAME)
        environment[ROOM_ACCESS_SECRET_KEY] = secret
        write_runtime_environment(root, environment)
        verify_runtime_environment(root / RUNTIME_ENV_NAME)
        chown_tree(root, int(record["uid"]), int(record["gid"]))
        record["roomAccessSecret"] = origin
        record["roomAccessSecretLength"] = len(secret)
        record["kind"] = "rehearsal-only; not final cutover backup; runtime.env refreshed, data untouched"
        record_path.write_text(json.dumps(record, indent=2))
        print(json.dumps(record))
        return

    current = json.loads(subprocess.check_output(["docker", "inspect", "musicparty-docker-test-music-party-1"]))[0]
    old_image = current["Image"]
    root.mkdir(mode=0o700)
    data = root / "data"
    cache = root / "cached_media"
    data.mkdir()
    cache.mkdir()
    mounts = {entry["Destination"]: pathlib.Path(entry["Source"]) for entry in current["Mounts"]}
    for item in mounts["/app/data"].iterdir():
        if item.name in ("musicparty.db", "musicparty.db-wal", "musicparty.db-shm", "musicparty.db-journal"):
            continue
        target = data / item.name
        if item.is_dir():
            shutil.copytree(item, target)
        elif item.is_file():
            shutil.copy2(item, target)
    subprocess.run(["docker", "run", "--rm", "--user", "0:0", "--network", "none", "--mount", f"type=bind,src={mounts['/app/data']},dst=/source,readonly", "--mount", f"type=bind,src={data},dst=/snapshot", "--entrypoint", "/app/dbsnapshot", old_image, "-source", "/source/musicparty.db", "-destination", "/snapshot/musicparty.db"], check=True)
    subprocess.run(["docker", "run", "--rm", "--user", "0:0", "--network", "none", "--mount", f"type=bind,src={data},dst=/snapshot,readonly", "--entrypoint", "/app/dbcheck", old_image, "-db", "/snapshot/musicparty.db"], check=True)
    environment = dict(entry.split("=", 1) for entry in current["Config"]["Env"])
    environment.update({"BASE_URL": "http://music-party-rehearsal:8080", "ALLOWED_ORIGINS": "http://music-party-rehearsal:8080,http://127.0.0.1:18848", "APP_ENV": "production", "AUTH_SECURE_COOKIES": "false"})
    for key in ("NODE_ENV", "ENV", "SPRING_PROFILES_ACTIVE"):
        environment.pop(key, None)
    secret, origin = resolve_room_access_secret(environment, root / RUNTIME_ENV_NAME)
    environment[ROOM_ACCESS_SECRET_KEY] = secret
    write_runtime_environment(root, environment)
    verify_runtime_environment(root / RUNTIME_ENV_NAME)
    uid, gid = (current["Config"].get("User") or "10001:10001").split(":")
    chown_tree(root, int(uid), int(gid))
    record = {"oldImageId": old_image, "dataSource": str(mounts["/app/data"]), "stagingData": str(data), "uid": uid, "gid": gid, "network": next(iter(current["NetworkSettings"]["Networks"])), "snapshotIntegrity": "ok", "roomAccessSecret": origin, "roomAccessSecretLength": len(secret), "kind": "rehearsal-only; not final cutover backup"}
    (root / RECORD_NAME).write_text(json.dumps(record, indent=2))
    print(json.dumps(record))


if __name__ == "__main__":
    main()
