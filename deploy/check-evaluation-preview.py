"""Exercise the isolated evaluation deployment, never a production endpoint."""
import argparse
import http.cookiejar
import json
import subprocess
import time
import urllib.error
import urllib.request
import uuid
from pathlib import Path

parser = argparse.ArgumentParser()
parser.add_argument('--verify-proof')
parser.add_argument('--proof-output', default='/tmp/incidentgpt-evaluation-proof.json')
args = parser.parse_args()
BASE = 'http://127.0.0.1:8195'
client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
def api(method, path, body=None, status=200, origin=BASE):
    headers = {'Origin': origin}
    data = None
    if body is not None:
        data = json.dumps(body).encode()
        headers['Content-Type'] = 'application/json'
    request = urllib.request.Request(BASE + '/api/v1' + path, data=data, headers=headers, method=method)
    try:
        response = client.open(request, timeout=10)
    except urllib.error.HTTPError as exc:
        response = exc
    raw = response.read()
    assert response.code == status, (method, path, response.code, raw[:180])
    return json.loads(raw) if raw and status < 400 else None

def calls():
    # Inspect the fixture inside its private network, independently of host NAT.
    command = ['docker', 'compose', '-p', 'incidentgpt-evaluation-preview', '-f',
               'deploy/evaluation-preview.compose.yaml', 'exec', '-T', 'provider',
               'python', '-c', "import urllib.request; print(urllib.request.urlopen('http://127.0.0.1:8080/calls', timeout=2).read().decode())"]
    return json.loads(subprocess.check_output(command, timeout=10, stderr=subprocess.PIPE))

for attempt in range(60):
    try:
        api('GET', '/auth/session', status=401)
        calls()  # A running container does not yet prove HTTP readiness.
        break
    except (OSError, AssertionError, subprocess.SubprocessError):
        time.sleep(1)
else:
    raise RuntimeError('isolated preview did not become ready')
api('GET', '/evaluation/config', status=401)
api('POST', '/auth/session', {'username': 'preview-operator', 'password': 'preview-only-password'})
for attempt in range(60):
    try:
        config = api('GET', '/evaluation/config')
        if config['enabled'] and len(config['profiles']) == 3:
            break
    except (OSError, AssertionError, subprocess.SubprocessError):
        pass
    time.sleep(1)
else:
    raise RuntimeError('worker profile discovery did not become ready')
if args.verify_proof:
    proof = json.loads(Path(args.verify_proof).read_text())
    case = api('GET', '/evaluation/cases/' + proof['case_id'])
    run = api('GET', '/evaluation/runs/' + proof['run_id'])
    assert case['snapshot_hash'] == proof['snapshot_hash']
    assert sorted(result['status'] for result in run['results']) == ['complete', 'complete', 'failed']
    alpha = next(result for result in run['results'] if result['profile']['id'] == 'alpha')
    assert alpha['review']['revision'] == 1 and alpha['review']['verdict'] == 'partial'
    assert len(calls()['calls']) == proof['provider_calls']
    assert len(calls()['telegram']) == proof['telegram_calls']
    print('PASS: case, snapshot, comparison and human review survive backend/worker restart; no new inference or Telegram sends')
    raise SystemExit()

state = api('GET', '/operations/ai-control')
api('PUT', '/operations/ai-control', {'enabled': False, 'revision': state['revision']})
before = calls()
case = api('POST', '/evaluation/cases', {'name': 'Preview: retained OOM observations', 'scenario': 'Synthetic incident. Container terminated with OOMKilled. Memory usage 510 MiB, limit 512 MiB. Node conditions unavailable.', 'expected_cause': 'DO_NOT_SEND_REFERENCE: container memory exhaustion; confirm node state.'}, status=201)
assert case['input']['evidence'][0]['excerpt'].startswith('Synthetic incident')
assert case['reference']['cause'].startswith('DO_NOT_SEND_REFERENCE')
payload = {'profile_ids': ['alpha', 'beta', 'invalid'], 'request_id': str(uuid.uuid4()), 'confirm_provider_calls': True}
run = api('POST', '/evaluation/cases/' + case['case_id'] + '/runs', payload, status=202)
again = api('POST', '/evaluation/cases/' + case['case_id'] + '/runs', payload, status=202)
assert again['run_id'] == run['run_id'] and again['duplicate']
api('POST', '/evaluation/cases/' + case['case_id'] + '/runs', {**payload, 'profile_ids': ['unlisted']}, status=400)
api('POST', '/evaluation/cases', {'name': 'foreign', 'scenario': 'test'}, status=403, origin='https://foreign.example')
time.sleep(2)
waiting = api('GET', '/evaluation/runs/' + run['run_id'])
assert waiting['paused'] and all(item['status'] == 'queued' for item in waiting['results'])
assert len(calls()['calls']) == len(before['calls'])
state = api('GET', '/operations/ai-control')
api('PUT', '/operations/ai-control', {'enabled': True, 'revision': state['revision']})
for attempt in range(45):
    result = api('GET', '/evaluation/runs/' + run['run_id'])
    if all(item['status'] in ('complete', 'failed') for item in result['results']):
        break
    time.sleep(1)
else:
    raise RuntimeError('comparison did not finish')
assert sorted(item['status'] for item in result['results']) == ['complete', 'complete', 'failed'], result
assert next(item for item in result['results'] if item['profile']['id'] == 'invalid')['error_code'] == 'worker_http_422'
for item in result['results']:
    if item['status'] == 'complete':
        assert item['result']['notification_status'] == 'disabled'
        assert item['result']['usage']['total_tokens'] == 120
        assert item['result']['usage']['cost'] == 0.00012
        assert item['result']['model'] == item['profile']['model']
review_path = '/evaluation/runs/' + run['run_id'] + '/results/alpha/review'
review = api('PUT', review_path, {'revision': 0, 'verdict': 'partial', 'note': 'Node observations are missing.'})
assert review['reviewed_by'] == 'preview-operator'
api('PUT', review_path, {'revision': 0, 'verdict': 'accurate', 'note': 'stale'}, status=409)
time.sleep(2)
after = calls()
new_calls = after['calls'][len(before['calls']):]
assert len(new_calls) == 3 and {item['model'] for item in new_calls} == {'mock/alpha', 'mock/beta', 'mock/invalid'}
assert not any(item['reference_leaked'] or item['tools'] for item in new_calls)
assert after['telegram'] == before['telegram']
proof = {'case_id': case['case_id'], 'run_id': run['run_id'], 'snapshot_hash': case['snapshot_hash'], 'provider_calls': len(after['calls']), 'telegram_calls': len(after['telegram']), 'checks': ['session ingress', 'allowlisted profiles', 'immutable snapshot', 'reference isolation', 'AI pause/resume', 'atomic comparison', 'idempotency', 'human review CAS', 'invalid citations fail independently', 'no paid retry', 'no Telegram']}
Path(args.proof_output).write_text(json.dumps(proof, indent=2) + '\n')
print('PASS: 3 explicit fixture calls, 2 valid results, 1 rejected citation, no reference leakage, no Telegram, no automatic retry')
print('Proof:', args.proof_output)
