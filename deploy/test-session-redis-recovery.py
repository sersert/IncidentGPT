"""Только disposable compose: Redis недоступен при старте backend, затем возвращается."""
import argparse
import http.cookiejar
import json
import subprocess
import time
import urllib.error
import urllib.request

parser = argparse.ArgumentParser()
parser.add_argument('--project', default='incidentgpt-login-preview')
parser.add_argument('--compose', default='deploy/login-preview.compose.yaml')
args = parser.parse_args()
command = ['docker', 'compose', '-p', args.project, '-f', args.compose]
base = 'http://127.0.0.1:8186'
client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))

def request(method='GET', path='/api/v1/auth/session', body=None):
    headers = {'Origin': base, 'Content-Type': 'application/json'}
    req = urllib.request.Request(base + path, method=method, headers=headers,
                                 data=json.dumps(body).encode() if body else None)
    try:
        response = client.open(req, timeout=10)
    except urllib.error.HTTPError as error:
        response = error
    response.read()
    return response.status

def login():
    return request('POST', body={'username': 'preview-operator', 'password': 'preview-only-password'})

assert login() == 200
try:
    subprocess.run(command + ['stop', 'redis'], check=True)
    subprocess.run(command + ['restart', 'backend'], check=True)
    # NewStorage fails at startup, while the independent session client must fail closed.
    for attempt in range(35):
        try:
            status = login()
        except (OSError, urllib.error.URLError):
            status = 0
        if status == 503:
            break
        time.sleep(.3)
    assert status == 503, status
    assert request(path='/api/v1/dashboard/stats') in (500, 503)
finally:
    subprocess.run(command + ['start', 'redis'], check=True)
for attempt in range(35):
    if login() == 200:
        break
    time.sleep(.3)
else:
    raise AssertionError('session Redis did not reconnect after late startup')
assert request(path='/api/v1/dashboard/stats') == 200
assert request('DELETE') == 204
print('PASS: configured Redis fails closed during startup loss and recovers without backend restart.')
