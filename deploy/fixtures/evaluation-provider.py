"""Deterministic preview-only provider; never forwards requests externally."""
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from datetime import datetime, timezone
calls = []
telegram = []
class Handler(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass
    def send(self, status, body):
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(json.dumps(body).encode())
    def do_GET(self):
        if self.path == '/calls':
            return self.send(200, {'calls': calls, 'telegram': telegram})
        self.send(200, {'ok': True})
    def do_POST(self):
        raw = self.rfile.read(int(self.headers.get('Content-Length', 0)))
        if self.path != '/v1/chat/completions':
            telegram.append(self.path)
            return self.send(200, {'ok': True, 'result': {'message_id': 1}})
        body = json.loads(raw)
        text = body['messages'][-1]['content']
        snapshot = json.loads(text.removeprefix('<untrusted_evidence>').removesuffix('</untrusted_evidence>'))
        calls.append({'model': body['model'], 'at': datetime.now(timezone.utc).isoformat(), 'evidence_ids': [e['evidence_id'] for e in snapshot['evidence']], 'reference_leaked': 'DO_NOT_SEND_REFERENCE' in raw.decode(), 'tools': 'tools' in body})
        if 'DO_NOT_SEND_REFERENCE' in raw.decode() or body.get('tools'):
            return self.send(400, {'error': 'reference or tools reached offline evaluation'})
        eid = snapshot['evidence'][0]['evidence_id']
        content = {'summary': 'Memory exhaustion is a hypothesis; verify node conditions.' if body['model'] == 'mock/alpha' else 'Compare a container memory limit with node pressure before deciding.', 'hypotheses': [{'text': 'Container reached its memory limit', 'supporting_evidence_ids': [eid], 'contradicting_evidence_ids': []}], 'missing_information': ['Node conditions'], 'suggested_next_checks': ['Review retained node observations']}
        if body['model'] == 'mock/invalid':
            content['hypotheses'][0]['supporting_evidence_ids'] = ['foreign-evidence']
        self.send(200, {'choices': [{'message': {'role': 'assistant', 'content': json.dumps(content)}}], 'usage': {'prompt_tokens': 80, 'completion_tokens': 40, 'total_tokens': 120, 'cost': 0.00012}})
ThreadingHTTPServer(('0.0.0.0', 8080), Handler).serve_forever()
