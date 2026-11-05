"""Проверка старой установки: только htpasswd, без reader Secret и session opt-in."""
import argparse
import base64
import json
import pathlib
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

parser = argparse.ArgumentParser()
parser.add_argument('--project', default='incidentgpt-login-preview')
parser.add_argument('--compose', default='deploy/login-preview.compose.yaml')
args = parser.parse_args()
compose = ['docker', 'compose', '-p', args.project, '-f', args.compose]
backend_name = args.project + '-basic-backend'
ui_name = args.project + '-basic-ui'
base = 'http://127.0.0.1:8187'

def request(path, authenticated=False):
    headers = {}
    if authenticated:
        credentials = base64.b64encode(b'preview-operator:preview-only-password').decode()
        headers['Authorization'] = 'Basic ' + credentials
    try:
        response = urllib.request.urlopen(urllib.request.Request(base + path, headers=headers), timeout=5)
    except urllib.error.HTTPError as error:
        response = error
    return response.status, response.read()

with tempfile.TemporaryDirectory(prefix='incidentgpt-basic-upgrade-') as scratch:
    password_hash = subprocess.check_output(['openssl', 'passwd', '-apr1', 'preview-only-password'], text=True).strip()
    htpasswd = pathlib.Path(scratch) / 'htpasswd'
    htpasswd.write_text('preview-operator:' + password_hash + '\n')
    htpasswd.chmod(0o644)
    backend_image = subprocess.check_output(compose + ['images', '-q', 'backend'], text=True).strip()
    ui_image = subprocess.check_output(compose + ['images', '-q', 'ui'], text=True).strip()
    assert backend_image and ui_image
    try:
        subprocess.run(['docker', 'run', '-d', '--name', backend_name, '--network', args.project + '_default',
                        '-e', 'FORWARD_TO_AI_WORKER=false', backend_image], check=True, stdout=subprocess.DEVNULL)
        subprocess.run(['docker', 'run', '-d', '--name', ui_name, '--network', args.project + '_default',
                        '-p', '127.0.0.1::80', '-e', 'NGINX_RESOLVER=127.0.0.11',
                        '-e', 'BACKEND_URL=http://' + backend_name + ':8080',
                        '-v', str(htpasswd) + ':/etc/nginx/secrets/htpasswd:ro', ui_image],
                       check=True, stdout=subprocess.DEVNULL)
        published = json.loads(subprocess.check_output(['docker', 'inspect', ui_name], text=True))
        port = published[0]['NetworkSettings']['Ports']['80/tcp'][0]['HostPort']
        base = 'http://127.0.0.1:' + port
        for attempt in range(45):
            try:
                if request('/healthz')[0] == 200:
                    break
            except (OSError, urllib.error.URLError):
                pass
            time.sleep(.3)
        else:
            raise AssertionError('Basic preview did not become ready')
        status, config = request('/auth-config.json')
        assert status == 200 and json.loads(config) == {'mode': 'basic'}
        for path in ['/', '/favicon.svg', '/api/v1/dashboard/stats']:
            assert request(path)[0] == 401, path
            assert request(path, True)[0] == 200, path
        assert request('/api/v1/auth/session', True)[0] == 404
        subprocess.run(['docker', 'exec', ui_name, 'nginx', '-t'], check=True)
        print('PASS: default Basic upgrade works with htpasswd and without backend reader Secret.')
    finally:
        subprocess.run(['docker', 'rm', '-f', ui_name, backend_name], check=False, stdout=subprocess.DEVNULL)
