# Leadomi SIP backups and restore

## Daily host backup

Install `scripts/business-backup.py` as `/opt/gosip-business-backup.py` and
`deploy/gosip-backup.service` / `.timer` under `/etc/systemd/system`. On the
existing deployment the default PBX container prefix is already configured.
For a fresh app, add a systemd service override:

```ini
[Service]
Environment=PBX_CONTAINER_PREFIX=leadomi-sip-pbx-
```

Use your actual prefix from `docker ps --format '{{.Names}}'` instead of this
direct-Compose example. Exactly one running PBX must match. Then:

```sh
sudo install -m 700 scripts/business-backup.py /opt/gosip-business-backup.py
sudo install -m 644 deploy/gosip-backup.service deploy/gosip-backup.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now gosip-backup.timer
sudo systemctl start gosip-backup.service
sudo systemctl status gosip-backup.timer
```

The job runs daily02:15UTC (07:45IST), with up to five minutes jitter. Root-only
archives in `/var/backups/gosip` retain30days. Each contains `gosip.db`,
`delivery.db`, and `credentials.json`, using SQLite's online backup API and
integrity checks. The first database includes users, routing, call/SMS history,
saved activity actors and configuration audit. PBX state includes offline SMS
delivery progress and dedicated outbound SIP credentials. Files are0600 and
the directory0700. The dashboard records the last successful host backup.

On-demand **Dashboard** database backups are separate and do not constitute a
full host/PBX recovery archive. Use the business job described here for
persistent database and PBX state. Source archives are documented separately
in [source-only backups](SOURCE_BACKUP.md).

## Off-server protection

No off-server destination is configured on the existing VPS. Arrange an
access-controlled external store and verify upload/retention independently.
A same-VPS backup cannot protect against loss of that VPS. Keep a protected
export of Coolify configuration/environment, certificate/DNS recovery access,
and the matching repository revision. The archive excludes Twilio API tokens,
the incoming-trunk secret and deployed TLS private keys.

## Restore validation and recovery

First extract only the expected three members into a private temporary
directory. Reject unexpected paths/symlinks, run SQLite `PRAGMA integrity_check`
against both database copies and validate credential JSON and permissions.
Perform this check without modifying live volumes.

For an actual incident:

1. Stop both app containers and preserve their current volumes before replacing
   data. Record the archive timestamp and Git revision.
2. Restore `gosip.db` into the Go data volume with ownership writable by UID1000
   and mode0600. Restore `delivery.db` and `credentials.json` into the private PBX
   volume, mode0600. Preserve media directories separately if recovering them.
3. Restore the matching Coolify environment and trusted certificate files.
4. Compare current Twilio number callbacks and credentials with the restored
   assignments. Provider state is external and is not rolled back by SQLite.
   Do not let a restored UI silently reassign any numbers.
5. Start services, verify health, trusted TLS, existing accounts and routes,
   reconnect phones, and perform manual call/audio/SMS acceptance tests.

Do not restore into a running application. Pause the backup timer during an
actual recovery and resume it only after verification. Activity records restored
from an older archive may omit later activity; carrier logs remain external.
