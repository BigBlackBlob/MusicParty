"""Snapshot a running deployment for rehearsal without changing its container or data."""
import json
import os
import pathlib
import shutil
import subprocess
import sys

root = pathlib.Path(sys.argv[1]).resolve()
if root.exists():
    raise RuntimeError("Isolation directory already exists")
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
for value in environment.values():
    if "\n" in value or "\r" in value:
        raise RuntimeError("Multiline environment value needs explicit provisioning")
(root / "runtime.env").write_text("".join(key + "=" + value + "\n" for key, value in environment.items()))
os.chmod(root / "runtime.env", 0o600)
uid, gid = (current["Config"].get("User") or "10001:10001").split(":")
for directory, folders, files in os.walk(root):
    os.chown(directory, int(uid), int(gid))
    for file in files:
        os.chown(pathlib.Path(directory) / file, int(uid), int(gid))
record = {"oldImageId": old_image, "dataSource": str(mounts["/app/data"]), "stagingData": str(data), "uid": uid, "gid": gid, "network": next(iter(current["NetworkSettings"]["Networks"])), "snapshotIntegrity": "ok", "kind": "rehearsal-only; not final cutover backup"}
(root / "rehearsal.json").write_text(json.dumps(record, indent=2))
print(json.dumps(record))
