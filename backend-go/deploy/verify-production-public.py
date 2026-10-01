"""Verify the authorized cutover through HTTPS without changing user content."""
import http.cookiejar
import json
import pathlib
import sys
import urllib.error
import urllib.parse
import urllib.request

root = pathlib.Path('/opt/musicparty-cutover-20261001')
origin = 'https://musicparty.nirotiy.top'
cookies = http.cookiejar.CookieJar()
client = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(cookies))
checks = {}
completed = False
logged_in = False


def call(path, method='GET', body=None, csrf=True, media=False):
    url = urllib.parse.urljoin(origin, path)
    if urllib.parse.urlparse(url).scheme != 'https' or urllib.parse.urlparse(url).netloc != urllib.parse.urlparse(origin).netloc:
        raise RuntimeError('Cross-origin request refused')
    headers = {'User-Agent': 'MusicParty-VPS-Validation/1.0', 'Origin': origin,
               'X-Desktop-API-Version': '2026-01', 'X-Desktop-Client-Version': '0.2.0'}
    if csrf:
        token = next((cookie.value for cookie in cookies if cookie.name == 'MP_CSRF'), None)
        if token:
            headers['X-CSRF-Token'] = token
    if media:
        headers['Range'] = 'bytes=0-1023'
    if body is not None:
        headers['Content-Type'] = 'application/json'
    request = urllib.request.Request(url, method=method, headers=headers,
                                     data=json.dumps(body).encode() if body is not None else None)
    try:
        response = client.open(request, timeout=45)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        payload = response.read(1024 if media else 1024 * 1024)
        value = json.loads(payload) if not media and response.headers.get_content_type() == 'application/json' and payload else payload
        return response.status, value, response.headers


def check(name, result):
    checks[name] = bool(result)
    if not result:
        raise RuntimeError(name)


try:
    check('publicReadiness', call('/actuator/health/readiness')[0] == 200)
    check('publicHomepage', call('/')[0] == 200)
    credentials = json.loads(pathlib.Path('/opt/musicparty-rehearsal-20261001/admin-credentials.json').read_text())
    status, account, headers = call('/api/account/login', 'POST', credentials)
    logged_in = status == 200
    check('publicAdminLogin', logged_in and account.get('isAdmin') is True)
    check('publicSessionSecureHttpOnly', any(cookie.name == 'MP_SESSION' and cookie.secure and cookie.has_nonstandard_attr('HttpOnly') for cookie in cookies))
    check('publicCsrfSecureReadable', any(cookie.name == 'MP_CSRF' and cookie.secure and not cookie.has_nonstandard_attr('HttpOnly') for cookie in cookies))
    check('publicSessionWorks', call('/api/account/me')[0] == 200)
    check('publicCsrfNegative', call('/api/account/logout', 'POST', {}, csrf=False)[0] == 403)
    status, sources, _ = call('/api/platforms')
    check('publicSourcesPreserved', status == 200 and sorted(entry['id'] for entry in sources) == ['bilibili', 'local', 'netease'])
    for platform, identifier in [('netease', '22672740'), ('bilibili', 'BV1pv6TYeEHL')]:
        status, resolved, _ = call(f'/api/desktop/v1/media/{platform}/{identifier}/resolve')
        check(platform + 'PublicResolve', status == 200 and isinstance(resolved.get('url'), str))
        status, payload, headers = call(resolved['url'], media=True)
        check(platform + 'PublicRange', status == 206 and len(payload) == 1024 and bool(headers.get('Content-Range')))
    completed = True
except Exception as error:
    checks['validationCompleted'] = False
    failure_type = type(error).__name__
finally:
    if logged_in:
        try:
            checks['publicLogout'] = call('/api/account/logout', 'POST', {})[0] == 204
        except Exception:
            checks['publicLogout'] = False
    report = {'checks': checks, 'completed': completed, 'passed': completed and all(checks.values()),
              'scope': 'production HTTPS login/logout, secure cookies, CSRF negative, enabled sources and 1024-byte Range; no content mutation'}
    if not completed:
        report['failureType'] = failure_type
    (root / 'validation-production-public.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report))
    sys.exit(0 if report['passed'] else 1)
