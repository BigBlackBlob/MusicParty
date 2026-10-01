"""Explicitly gated MusicParty maintenance switch with a stopped-writer snapshot."""
import hashlib
import json
import os
import pathlib
import shutil
import sqlite3
import subprocess
import sys
import time
import urllib.error
import urllib.request

root = pathlib.Path('/opt/musicparty-cutover-20261001')
plan = json.loads((root / 'plan.json').read_text())
base = pathlib.Path(plan['compose'])
data = pathlib.Path(plan['mounts']['/app/data'])
cache = pathlib.Path(plan['mounts']['/app/cached_media'])
if (base, data, cache) != (pathlib.Path('/opt/musicparty-docker-test/docker-compose.yml'), pathlib.Path('/opt/musicparty-docker-test/music_party/data'), pathlib.Path('/opt/musicparty-docker-test/music_party/cached_media')):
    raise RuntimeError('Unexpected production paths')
for override in ('new-image.json', 'old-image.json'):
    subprocess.run(['docker', 'compose', '-f', str(base), '-f', str(root / override), 'config', '--quiet'], cwd=base.parent, check=True)
if sys.argv[1:] == ['--preflight']:
    current = json.loads(subprocess.check_output(['docker', 'inspect', plan['container']]))[0]
    expected = json.loads(subprocess.check_output(['docker', 'image', 'inspect', plan['oldImage']]))[0]['Id']
    if current['Image'] != expected or not current['State']['Running']:
        raise RuntimeError('Production changed since plan')
    print(json.dumps({'preflight': True, 'productionUntouched': True, 'newImage': plan['newImage']}))
    sys.exit(0)
if sys.argv[1:] != ['--execute'] or os.environ.get('MUSICPARTY_MAINTENANCE_CONFIRMED') != 'YES':
    raise RuntimeError('Explicit maintenance approval is required')
if not plan['readyForMaintenanceApproval']:
    raise RuntimeError('Unresolved environment drift')


def command(args):
    subprocess.run(args, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def compose(override, *args):
    command(['docker', 'compose', '-f', str(base), '-f', str(root / override), *args, plan['service']])


def dbcheck(directory, image):
    command(['docker', 'run', '--rm', '--user', '0:0', '--read-only', '--network', 'none', '--mount', f'type=bind,src={directory},dst=/snapshot,readonly', '--entrypoint', '/app/dbcheck', image, '-db', '/snapshot/musicparty.db'])


def health():
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    for _ in range(60):
        try:
            with client.open('http://127.0.0.1:8848/actuator/health/readiness', timeout=5) as response:
                if response.status == 200:
                    return True
        except (urllib.error.URLError, ConnectionError):
            pass
        time.sleep(1)
    return False


def counts(database):
    with sqlite3.connect(f'file:{database}?mode=ro', uri=True) as connection:
        tables = {row[0] for row in connection.execute("SELECT name FROM sqlite_master WHERE type='table'")}
        required = {'user_account', 'room', 'local_track', 'user_playlist', 'user_playlist_track', 'room_playlist', 'room_playlist_track'}
        if not required.issubset(tables):
            raise RuntimeError('Business count tables missing')
        return {table: connection.execute('SELECT count(*) FROM "' + table + '"').fetchone()[0] for table in sorted(required)}


timestamp = time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
snapshot = root / ('snapshot-' + timestamp)
snapshot.mkdir(mode=0o700)
current = json.loads(subprocess.check_output(['docker', 'inspect', plan['container']]))[0]
expected = json.loads(subprocess.check_output(['docker', 'image', 'inspect', plan['oldImage']]))[0]['Id']
effective = json.loads(subprocess.check_output(['docker', 'compose', '-f', str(base), 'config', '--format', 'json'], cwd=base.parent))['services'][plan['service']]
runtime = dict(value.split('=', 1) for value in current['Config']['Env'])
if current['Image'] != expected or not current['State']['Running'] or any(runtime.get(key) != str(value) for key, value in effective.get('environment', {}).items()):
    raise RuntimeError('Production changed since plan; no service stopped')
if shutil.disk_usage(root).free < sum(item.stat().st_size for folder in (data, cache) for item in folder.rglob('*') if item.is_file()) + 512 * 1024 * 1024:
    raise RuntimeError('Insufficient backup space; no service stopped')
stopped = False
new_started = False
snapshot_ready = False
record = {'snapshot': str(snapshot), 'oldImage': plan['oldImage'], 'newImage': plan['newImage'], 'passed': False, 'rolledBack': False}
try:
    compose('old-image.json', 'stop', '--timeout', '30')
    stopped = True
    if json.loads(subprocess.check_output(['docker', 'inspect', plan['container']]))[0]['State']['Running']:
        raise RuntimeError('Production writer did not stop')
    target_data = snapshot / 'data'
    target_data.mkdir()
    command(['docker', 'run', '--rm', '--user', '0:0', '--network', 'none', '--mount', f'type=bind,src={data},dst=/source,readonly', '--mount', f'type=bind,src={target_data},dst=/snapshot', '--entrypoint', '/app/dbsnapshot', plan['oldImage'], '-source', '/source/musicparty.db', '-destination', '/snapshot/musicparty.db'])
    dbcheck(target_data, plan['oldImage'])
    for item in data.iterdir():
        if item.name in ('musicparty.db', 'musicparty.db-wal', 'musicparty.db-shm', 'musicparty.db-journal'):
            continue
        destination = target_data / item.name
        if item.is_dir():
            shutil.copytree(item, destination, symlinks=True)
        else:
            shutil.copy2(item, destination, follow_symlinks=False)
    shutil.copytree(cache, snapshot / 'cached_media', symlinks=True)
    record['databaseSha256'] = hashlib.sha256((target_data / 'musicparty.db').read_bytes()).hexdigest()
    record['beforeCounts'] = counts(target_data / 'musicparty.db')
    manifest = {'integrity_check': 'ok', 'databaseSha256': record['databaseSha256'], 'oldImage': plan['oldImage'], 'createdUtc': timestamp, 'counts': record['beforeCounts'], 'nonSqliteDataAndCacheCopiedWhileStopped': True}
    (snapshot / 'MANIFEST.txt').write_text(json.dumps(manifest, indent=2) + '\n')
    snapshot_ready = True
    compose('new-image.json', 'up', '-d', '--no-deps')
    new_started = True
    if not health():
        raise RuntimeError('New image readiness failed')
    dbcheck(data, plan['newImage'])
    record['afterCounts'] = counts(data / 'musicparty.db')
    if record['afterCounts'] != record['beforeCounts']:
        raise RuntimeError('Persistent business counts changed')
    record['passed'] = True
except Exception as error:
    record['failureType'] = type(error).__name__
    if stopped:
        compose('new-image.json', 'stop', '--timeout', '30')
        if snapshot_ready:
            for active, name in ((data, 'data'), (cache, 'cached_media')):
                failure_copy = active.with_name(active.name + '.failed-' + timestamp)
                if failure_copy.exists():
                    raise RuntimeError('Failure archive exists; refusing overwrite')
                active.rename(failure_copy)
                shutil.copytree(snapshot / name, active, symlinks=True)
                for directory, folders, files in os.walk(active, followlinks=False):
                    os.chown(directory, 10001, 10001)
                    for filename in files:
                        os.chown(pathlib.Path(directory) / filename, 10001, 10001, follow_symlinks=False)
        compose('old-image.json', 'up', '-d', '--no-deps')
        record['rolledBack'] = health()
finally:
    (snapshot / 'cutover-result.json').write_text(json.dumps(record, indent=2) + '\n')
    os.chmod(snapshot / 'cutover-result.json', 0o600)
    print(json.dumps(record))
sys.exit(0 if record['passed'] else 1)
