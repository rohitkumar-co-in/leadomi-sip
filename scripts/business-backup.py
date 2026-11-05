#!/usr/bin/env python3
"""Root-only daily backups of Leadomi SIP SQLite and private Asterisk state.

Uses SQLite's online backup API; never copies a live database or prints secrets.
Restore into a separate directory and verify before touching running volumes.
Coolify's environment and TLS certificate recovery are managed separately.
"""
import datetime
import json
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile

os.umask(0o077)
if os.geteuid() != 0:
    raise SystemExit('Run this backup as root')
root = Path('/var/backups/gosip')
root.mkdir(mode=0o700, parents=True, exist_ok=True)
root.chmod(0o700)
names = subprocess.check_output(['docker', 'ps', '--format', '{{.Names}}'], text=True).splitlines()
prefix = os.getenv('PBX_CONTAINER_PREFIX', 'pbx-zgzcndvsggqprdlnm503jmpu')
matches = [n for n in names if n.startswith(prefix)]
if len(matches) != 1:
    raise SystemExit('Expected exactly one production PBX container')
pbx = matches[0]
code = '''import sqlite3,os,shutil,json
from pathlib import Path
os.umask(0o077)
stage=Path('/var/lib/gosip-pbx/backup-stage');stage.mkdir(exist_ok=True)
for name,source in [('gosip.db','file:/gosip/gosip.db?mode=ro'),('delivery.db','/var/lib/gosip-pbx/delivery.db')]:
 target=stage/name;target.unlink(missing_ok=True)
 original=sqlite3.connect(source,uri=True);backup=sqlite3.connect(target)
 original.backup(backup);assert backup.execute('PRAGMA integrity_check').fetchone()[0]=='ok'
 backup.close();original.close();target.chmod(0o600)
credentials=json.loads(os.getenv('PBX_TWILIO_DEVICE_PASSWORDS') or '{}')
credentials[os.environ['PBX_TWILIO_USER']]=os.environ['PBX_TWILIO_PASSWORD']
path=Path('/var/lib/gosip-pbx/credentials.json')
if path.exists():credentials.update(json.loads(path.read_text()))
(stage/'credentials.json').write_text(json.dumps(credentials));(stage/'credentials.json').chmod(0o600)
'''
subprocess.run(['docker', 'exec', pbx, 'python3', '-c', code], check=True)
now = datetime.datetime.now(datetime.timezone.utc)
destination = root / ('gosip-' + now.strftime('%Y%m%d-%H%M%S') + '.tar.gz')
with tempfile.TemporaryDirectory(dir=root) as temp:
    subprocess.run(['docker', 'cp', pbx + ':/var/lib/gosip-pbx/backup-stage/.', temp], check=True, stdout=subprocess.DEVNULL)
    with tarfile.open(destination, 'w:gz') as archive:
        for name in ('gosip.db', 'delivery.db', 'credentials.json'):
            archive.add(Path(temp) / name, arcname=name)
destination.chmod(0o600)
# Only this backup job's archives in its fixed private directory are retained.
cutoff = now.timestamp() - 30 * 86400
for archive in root.glob('gosip-*.tar.gz'):
    if archive.is_file() and archive.stat().st_mtime < cutoff:
        archive.unlink()
status = json.dumps({'last_success': now.isoformat(), 'retention_days': 30})
subprocess.run(['docker', 'exec', pbx, 'python3', '-c',
    "from pathlib import Path;import sys;p=Path('/var/lib/gosip-pbx/backup-status.json');p.write_text(sys.argv[1]);p.chmod(0o600)", status], check=True)
print('Backup created; both SQLite integrity checks passed:', destination.name)
