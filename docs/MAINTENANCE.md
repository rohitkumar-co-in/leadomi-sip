# Leadomi SIP maintenance and release procedure

This document describes maintenance of the implemented business system rather
than a legacy roadmap. Do not present retained experimental code as a shipped
feature. Preserve provider connections, phone passwords and existing volumes.

1. Read the current operational/API guides and capture a verified backup.
2. Implement changes with explicit number-review boundaries and no startup
   provider writes. Cover provisioning guards, session revocation, attribution
   snapshots and private bridge behavior with meaningful tests.
3. Build Vue/TypeScript; run Go API/DB/SIP/startup tests; use isolated fake-provider
   and PBX integration tests without paid calls/SMS. Scan reachable dependency
   vulnerabilities when changing dependencies.
4. Push the reviewed revision, deploy to the existing app, verify service health,
   read-only callback mappings and existing SIP authentication. Reconnect phones.
5. Let the operator perform paid manual acceptance tests. Verify backups after
   deployment and keep external recovery copies. Document remaining limitations.

See [Coolify](COOLIFY_DEPLOYMENT.md),
[backups](BACKUP.md), [API](API.md).
