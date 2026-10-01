"""Prepare immutable Compose overrides and detect drift without stopping production."""
import json
import os
import pathlib
import subprocess
import sys

container = 'musicparty-docker-test-music-party-1'
base = pathlib.Path('/opt/musicparty-docker-test/docker-compose.yml')
root = pathlib.Path('/opt/musicparty-cutover-20261001')
image = 'crpi-533x5q1t88ew0x21.cn-hangzhou.personal.cr.aliyuncs.com/nrt-base/nrt-music-party@sha256:259275158661e677e7bc30b90c0377694bf1cf8e46742477f2a6458ca8334cb1'
current = json.loads(subprocess.check_output(['docker', 'inspect', container]))[0]
service = current['Config']['Labels']['com.docker.compose.service']
config = json.loads(subprocess.check_output(['docker', 'compose', '-f', str(base), 'config', '--format', 'json'], cwd=base.parent))
effective = config['services'][service]
runtime = dict(value.split('=', 1) for value in current['Config']['Env'])
drift = sorted(key for key, value in effective.get('environment', {}).items() if runtime.get(key) != str(value))
old = json.loads(subprocess.check_output(['docker', 'image', 'inspect', current['Image']]))[0]
old_digest = next(value for value in old['RepoDigests'] if value.endswith('sha256:98461b8035270f238d1cfd2fb8d26a9fd0a7ce57f5459a46ddb1ad5ffdf2cbdd'))
new = json.loads(subprocess.check_output(['docker', 'image', 'inspect', image]))[0]
revision = new['Config']['Labels']['org.opencontainers.image.revision']
if revision != '8a989f23c9d718ddaaffb214714f332bd5e36a80':
    raise RuntimeError('Unexpected new image revision')
root.mkdir(mode=0o700, exist_ok=True)
for name, reference in [('new-image.json', image), ('old-image.json', old_digest)]:
    target = root / name
    value = json.dumps({'services': {service: {'image': reference}}}, indent=2) + '\n'
    if target.exists() and target.read_text() != value:
        raise RuntimeError('Existing override differs; do not overwrite')
    target.write_text(value)
    os.chmod(target, 0o600)
mounts = {entry['Destination']: entry['Source'] for entry in current['Mounts']}
plan = {
    'container': container, 'compose': str(base), 'service': service,
    'oldImage': old_digest, 'newImage': image, 'newRevision': revision,
    'mounts': mounts, 'runtimeUser': current['Config']['User'],
    'environmentDriftKeys': drift, 'baseUrl': runtime.get('BASE_URL'),
    'secureCookiesSetting': runtime.get('AUTH_SECURE_COOKIES'),
    'appEnv': runtime.get('APP_ENV'), 'productionStillRunning': current['State']['Running'],
    'switchAuthorized': False, 'readyForMaintenanceApproval': not drift,
    'sequence': ['user approval', 'stop the sole production writer', 'snapshot SQLite with dbsnapshot and dbcheck', 'copy non-SQLite data and media while stopped', 'start the same Compose service with new-image.json', 'verify health, counts, HTTPS cookies and enabled sources'],
    'rollback': ['stop new writer', 'preserve failed data', 'restore verified database plus data/media snapshot', 'start same service with old-image.json', 'verify health'],
}
(root / 'plan.json').write_text(json.dumps(plan, indent=2) + '\n')
os.chmod(root / 'plan.json', 0o600)
print(json.dumps(plan))
sys.exit(0 if not drift else 1)
