"""Exercise only the independent rehearsal's auth, upload and persistence paths."""
import http.cookiejar
import io
import json
import math
import pathlib
import sqlite3
import struct
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import wave

root = pathlib.Path("/opt/musicparty-rehearsal-20261001")
base = "http://127.0.0.1:18848"
cookies = http.cookiejar.CookieJar()
client = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(cookies))
credentials_file = root / "admin-credentials.json"
credentials_file.chmod(0o600)
credentials = json.loads(credentials_file.read_text())
checks = {}
providers = []
completed = False


def request(path, method="GET", body=None, csrf=True, content_type="application/json", extra_headers=None, read_limit=None):
    headers = {"Origin": base, "X-Desktop-API-Version": "2026-01", "X-Desktop-Client-Version": "0.2.0"}
    if csrf:
        value = next((cookie.value for cookie in cookies if cookie.name == "MP_CSRF"), None)
        if value:
            headers["X-CSRF-Token"] = value
    if body is not None:
        headers["Content-Type"] = content_type
        if not isinstance(body, bytes):
            body = json.dumps(body).encode()
    if extra_headers:
        headers.update(extra_headers)
    url = urllib.parse.urljoin(base, path)
    if urllib.parse.urlparse(url).netloc != urllib.parse.urlparse(base).netloc:
        headers = {key: value for key, value in headers.items() if key not in ("Origin", "X-CSRF-Token")}
    req = urllib.request.Request(url, data=body, headers=headers, method=method)
    try:
        response = client.open(req, timeout=45)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        data = response.read(read_limit) if read_limit else response.read()
        value = json.loads(data) if response.headers.get_content_type() == "application/json" and data else data
        return response.status, value, dict(response.headers)


def check(name, condition):
    checks[name] = bool(condition)
    print(json.dumps({"check": name, "passed": bool(condition)}), flush=True)
    if not condition:
        raise RuntimeError(name)


playlist_id = None
track_id = None
try:
    check("readiness", request("/actuator/health/readiness")[0] == 200)
    check("anonymousAdminDenied", request("/api/local/tracks")[0] == 403)
    status, user, _ = request("/api/account/login", "POST", credentials)
    check("adminLogin", status == 200 and user.get("isAdmin") is True)
    check("privateSessionCookie", any(cookie.name == "MP_SESSION" and cookie.has_nonstandard_attr("HttpOnly") for cookie in cookies))
    check("csrfNegative", request("/api/me/playlists", "POST", {"name": "VPS rejected probe"}, csrf=False)[0] == 403)
    status, playlist, _ = request("/api/me/playlists", "POST", {"name": "VPS persistence probe"})
    playlist_id = playlist.get("id") if isinstance(playlist, dict) else None
    check("playlistCreate", status == 200 and bool(playlist_id))

    audio = io.BytesIO()
    with wave.open(audio, "wb") as output:
        output.setnchannels(1)
        output.setsampwidth(2)
        output.setframerate(16000)
        output.writeframes(b"".join(struct.pack("<h", round(2000 * math.sin(2 * math.pi * 440 * sample / 16000))) for sample in range(32000)))
    boundary = "watchparty-vps-rehearsal-upload"
    payload = (f'--{boundary}\r\nContent-Disposition: form-data; name="title"\r\n\r\nVPS upload probe\r\n--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="vps-probe.wav"\r\nContent-Type: audio/wav\r\n\r\n'.encode() + audio.getvalue() + f"\r\n--{boundary}--\r\n".encode())
    status, uploaded, _ = request("/api/local/tracks/upload", "POST", payload, content_type="multipart/form-data; boundary=" + boundary)
    track_id = uploaded.get("track", {}).get("id") if isinstance(uploaded, dict) else None
    check("uploadAccepted", status in (200, 201) and bool(track_id) and uploaded.get("duplicate") is False)
    deadline = time.monotonic() + 90
    track = None
    while time.monotonic() < deadline:
        _, tracks, _ = request("/api/local/tracks")
        track = next((entry for entry in tracks if entry.get("id") == track_id), None)
        if track and str(track.get("status", "")).lower() in ("ready", "completed", "failed", "error"):
            break
        time.sleep(1)
    check("uploadTranscoded", track is not None and str(track.get("status", "")).lower() in ("ready", "completed"))
    status, data, headers = request("/api/local/media/" + track_id, extra_headers={"Range": "bytes=0-1023"}, read_limit=1024)
    check("localRange", status == 206 and len(data) == 1024 and "Content-Range" in headers)
    status, source_list, _ = request("/api/platforms")
    check("enabledSourcesPreserved", status == 200 and sorted(entry["id"] for entry in source_list) == ["bilibili", "local", "netease"])
    subprocess.run(["docker", "restart", "music-party-rehearsal"], check=True, stdout=subprocess.DEVNULL)
    for attempt in range(30):
        try:
            if request("/actuator/health/readiness")[0] == 200:
                break
        except (urllib.error.URLError, ConnectionError):
            pass
        time.sleep(1)
    _, playlists, _ = request("/api/me/playlists")
    _, tracks, _ = request("/api/local/tracks")
    check("playlistPersistsAfterRestart", any(entry.get("id") == playlist_id for entry in playlists))
    check("uploadPersistsAfterRestart", any(entry.get("id") == track_id for entry in tracks))
    with sqlite3.connect(f"file:{root / 'data/musicparty.db'}?mode=ro", uri=True) as database:
        check("databaseIntegrity", database.execute("PRAGMA integrity_check").fetchone()[0] == "ok" and not database.execute("PRAGMA foreign_key_check").fetchall())
    completed = True
except Exception as error:
    print(json.dumps({"validationFailed": type(error).__name__}), flush=True)
finally:
    if track_id:
        checks["uploadCleanup"] = request("/api/local/tracks/" + track_id, "DELETE")[0] == 200
    if playlist_id:
        checks["playlistCleanup"] = request("/api/me/playlists/" + playlist_id, "DELETE")[0] == 200
    report = {"passed": completed and all(checks.values()), "completed": completed, "checks": checks, "scope": "independent rehearsal, source playback and browser/desktop sync still require separate checks"}
    (root / "validation-rehearsal.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report), flush=True)
    sys.exit(0 if report["passed"] else 1)
