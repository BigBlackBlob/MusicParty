"""Rehearse the exact production environment on copied data, never production mounts."""
import http.cookiejar
import json
import os
import pathlib
import subprocess
import sys
import time
import urllib.error
import urllib.request

root = pathlib.Path('/opt/musicparty-rehearsal-20261001')
temporary = 'music-party-rehearsal-secure'
image = 'crpi-533x5q1t88ew0x21.cn-hangzhou.personal.cr.aliyuncs.com/nrt-base/nrt-music-party@sha256:259275158661e677e7bc30b90c0377694bf1cf8e46742477f2a6458ca8334cb1'
production = json.loads(subprocess.check_output(['docker', 'inspect', 'musicparty-docker-test-music-party-1']))[0]
checks = {}
completed = False
created = False
stopped = False


def command(args):
    subprocess.run(args, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def check(name, result):
    checks[name] = bool(result)
    if not result:
        raise RuntimeError(name)


def call(path, method='GET', body=None, cookie=None, csrf=None):
    headers = {'Origin': 'https://musicparty.nirotiy.top', 'X-Forwarded-Proto': 'https', 'Content-Type': 'application/json'}
    if cookie:
        headers['Cookie'] = cookie
    if csrf:
        headers['X-CSRF-Token'] = csrf
    request = urllib.request.Request('http://127.0.0.1:18849' + path, method=method, headers=headers, data=json.dumps(body).encode() if body is not None else None)
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    try:
        response = client.open(request, timeout=15)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        payload = response.read()
        return response.status, json.loads(payload) if payload else None, response.headers


try:
    private = root / 'production-runtime.env'
    private.write_text('\n'.join(production['Config']['Env']) + '\n')
    os.chmod(private, 0o600)
    command(['docker', 'stop', 'music-party-rehearsal'])
    stopped = True
    command(['docker', 'run', '-d', '--name', temporary, '--network', 'musicparty-docker-test_music-net', '--user', '10001:10001', '--env-file', str(private), '--publish', '127.0.0.1:18849:8080', '--mount', f'type=bind,src={root / "data"},dst=/app/data', '--mount', f'type=bind,src={root / "cached_media"},dst=/app/cached_media', image])
    created = True
    for _ in range(30):
        try:
            if call('/actuator/health/readiness')[0] == 200:
                break
        except (urllib.error.URLError, ConnectionError):
            pass
        time.sleep(1)
    check('exactProductionEnvironmentStarts', call('/actuator/health/readiness')[0] == 200)
    credentials = json.loads((root / 'admin-credentials.json').read_text())
    status, user, headers = call('/api/account/login', 'POST', credentials)
    check('productionOriginAdminLogin', status == 200 and user.get('isAdmin') is True)
    cookies = headers.get_all('Set-Cookie')
    session = next(value for value in cookies if value.startswith('MP_SESSION='))
    csrf = next(value for value in cookies if value.startswith('MP_CSRF='))
    check('productionSessionSecureHttpOnly', '; secure' in session.lower() and '; httponly' in session.lower())
    check('productionCsrfSecure', '; secure' in csrf.lower() and '; httponly' not in csrf.lower())
    # This is the backend behind the existing TLS-terminating Tunnel, not a public HTTP login.
    cookie = '; '.join(value.split(';', 1)[0] for value in cookies)
    csrf_value = csrf.split(';', 1)[0].split('=', 1)[1]
    check('productionOriginSessionWorks', call('/api/account/me', cookie=cookie)[0] == 200)
    check('productionCsrfRefusesMissing', call('/api/account/logout', 'POST', {}, cookie=cookie)[0] == 403)
    status, sources, _ = call('/api/platforms', cookie=cookie)
    check('productionSourcesPreserved', status == 200 and sorted(value['id'] for value in sources) == ['bilibili', 'local', 'netease'])
    check('productionLogout', call('/api/account/logout', 'POST', {}, cookie=cookie, csrf=csrf_value)[0] == 204)
    completed = True
except Exception as error:
    checks['validationCompleted'] = False
finally:
    if created:
        command(['docker', 'stop', temporary])
        command(['docker', 'rm', temporary])
    if stopped:
        command(['docker', 'start', 'music-party-rehearsal'])
    current = json.loads(subprocess.check_output(['docker', 'inspect', 'musicparty-docker-test-music-party-1']))[0]
    checks['productionContainerUnchanged'] = current['Id'] == production['Id'] and current['Image'] == production['Image'] and current['State']['Running']
    report = {'checks': checks, 'completed': completed, 'passed': completed and all(checks.values()), 'scope': 'exact production environment on independent data; backend secure cookie contract; actual production Tunnel only after authorized cutover'}
    (root / 'validation-production-config.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report))
    sys.exit(0 if report['passed'] else 1)
