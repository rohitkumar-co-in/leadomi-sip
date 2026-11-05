# Leadomi SIP

An internal business phone console that connects Android SIP phones to owned
Twilio numbers. Asterisk handles SIP and audio; the Go backend handles Twilio
webhooks, SMS, administrator access, and persistent history. The Vue console
uses Leadomi's charcoal/orange theme with locally hosted DM Sans, Syne, and
JetBrains Mono fonts.

Repository: https://github.com/rohitkumar-co-in/leadomi-sip

## What works

- Inbound and outbound telephone calls over TLS SIP and SRTP audio.
- SIP MESSAGE ↔ Twilio SMS bridging, including an offline inbound queue.
- One SIP user, one assigned existing Twilio number, one registered phone.
- Custom SIP usernames and custom or generated phone passwords.
- Automatic dedicated Twilio SIP credential creation after an explicit number
  assignment review. Ordinary user edits preserve existing provider connections.
- Read-only inventory and connection checks; no number purchase, porting, or
  automatic reassignment on startup, login, refresh, or deployment.
- Persistent excluded-number protection with no force bypass, and deletion of
  disabled SIP users while retaining numbers and historical records.
- Password reset, user disabling, international destination restrictions,
  SMS submission limits, and two concurrent outbound calls per SIP user.
- Administrator-only console, call/SMS activity log, voicemail, local call
  routing, configuration audit, health checks, and database backup controls.
- Daily host backups with 30-day retention when the supplied systemd job is
  installed. Off-server storage must be arranged separately.

Number review detects Twilio voice/SMS webhooks, TwiML application links,
Messaging Service membership, and local routes. It cannot detect every external
system that uses a number solely for outbound traffic. An active assignment to
another local user is blocked. Messaging Services with their own inbound
handler are blocked rather than changed automatically. Explicitly assigning a
number replaces its webhooks/application links and local routes: review first.

## Start here

1. [Fresh installation](docs/INSTALLATION.md): infrastructure, credentials,
   certificates, first administrator, and server metadata.
2. [Coolify deployment](docs/COOLIFY_DEPLOYMENT.md): runtime variables, volumes,
   deployment, verification, and preserving an existing installation.
3. [Administrator guide](docs/ADMINISTRATION.md): create users, assign numbers,
   reset passwords, disable users, and manage access.
4. [Phone and console guide](docs/USER_GUIDE.md): Linphone, calling, SMS,
   history, routing, and troubleshooting.
5. [Operations and recovery](docs/BUSINESS_OPERATIONS.md),
   [backups](docs/BACKUP.md), [configuration](docs/CONFIGURATION.md), and
   [supported API](docs/API.md).

## Architecture and boundaries

Android Linphone → Asterisk → Twilio Voice; Linphone SIP MESSAGE → Asterisk
private HTTP bridge → backend → Twilio SMS API. Signed Twilio webhooks deliver
incoming calls/SMS and status updates. HTTPS serves the Vue administrator
console. SQLite and private PBX state persist in separate Docker volumes.

The supported production deployment is `docker-compose.coolify.yml` or
`docker-compose.production.yml`. The legacy standalone Go SIP server, manual
trunk/device provisioning, browser WebRTC, call-control UI, conferencing,
transfers, read receipts, and typing indicators are not supported product
features of this Asterisk deployment. MMS media is retained where Twilio
provides it, but mobile softphone MMS delivery is not promised.

This is a single-VPS system. It does not provide high availability or a monetary
spending cap. Android background restrictions can affect ringing and messages.
Use manual paid call/SMS tests after changes and keep off-server backups.

## Development

Backend module: `github.com/rohitkumar-co-in/leadomi-sip`. Use Go 1.26 with CGO and a C
compiler/SQLite build dependencies. Frontend: Node 24 and pnpm 10.11.0.

```sh
go test ./internal/api ./internal/db ./pkg/sip ./cmd/gosip
cd frontend
pnpm install --frozen-lockfile
pnpm build
```

See [architecture](docs/ARCHITECTURE.md) and
[security notes](docs/SECURITY.md). Runtime `GOSIP_*` variable names,
the `gosip` SIP digest realm, binary paths, and existing volume names are retained
for deployment compatibility; they are not the product brand.

See [project structure and development](docs/PROJECT_STRUCTURE.md),
[maintenance](docs/MAINTENANCE.md), and [source-only backups](docs/SOURCE_BACKUP.md).
