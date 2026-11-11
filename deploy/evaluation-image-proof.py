"""Record content IDs and source labels of images in the owned preview project."""
import json
import subprocess
compose = ['docker', 'compose', '-p', 'incidentgpt-evaluation-preview', '-f', 'deploy/evaluation-preview.compose.yaml']
ids = subprocess.check_output(compose + ['images', '-q'], text=True).split()
images = json.loads(subprocess.check_output(['docker', 'image', 'inspect', *sorted(set(ids))], text=True))
print(json.dumps({'commit': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(), 'images': [{'id': image['Id'], 'repo_digests': image.get('RepoDigests', []), 'source_revision': (image['Config'].get('Labels') or {}).get('org.opencontainers.image.revision')} for image in images]}, indent=2))
