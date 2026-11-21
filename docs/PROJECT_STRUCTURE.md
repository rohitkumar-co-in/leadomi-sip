# Leadomi SIP project index

| Location | Responsibility |
|---|---|
| `cmd/gosip` | Application startup |
| `internal/api` | Auth, signed webhooks, business workflows/history |
| `internal/db/migrations` | Embedded SQLite schema migrations |
| `internal/config` | Stable runtime environment handling |
| `pkg/sip` | Compatibility SIP utilities and legacy standalone implementation |
| `deploy/asterisk` | Production SIP/audio and MESSAGE bridge |
| `frontend/src/views` | Leadomi administrator console |
| `scripts/business-backup.py` | Root-only daily online backups |
| `docker-compose.coolify.yml` | Existing Coolify deployment |
| `docker-compose.production.yml` | Fresh direct Compose deployment |
| `docs` | Current installation, configuration, user/API/operations guides |

Start with [README](../README.md) or the [documentation index](README.md).

## Source layout

Keep the application entry point, Go module/lockfile, Dockerfile and production
Compose definitions at the root. Coolify uses those stable paths. Deployment
support lives in `deploy/`; reusable maintenance and verification commands in
`scripts/`; all current guides in `docs/`. Backend tests stay beside the Go
packages. The frontend regression test lives in `frontend/tests/`.

Generated `bin/`, `frontend/dist/`, `frontend/node_modules/`, TypeScript build
metadata and Python caches are disposable and ignored. Runtime `data/`,
recordings, backups, private keys and populated environment files are local or
volume state, outside committed source. Dependencies, font assets/licences and
SQL migrations are required source inputs and must remain in source backups.

## Development and verification

Use Go 1.26, a C compiler for SQLite/CGO, Node 24 and pnpm 10.11.0. From the
repository root, run `go mod download`. In `frontend`, run
`pnpm install --frozen-lockfile`, `pnpm test` and `pnpm build`.
Run `go vet ./...` and `go test -race ./...` from the root.

For development, start `go run ./cmd/gosip` and `pnpm dev` (inside `frontend`)
in separate terminals. Vite serves port 3000 and proxies `/api` to the backend
on localhost:8080. Supply local environment variables as documented in
[configuration](CONFIGURATION.md); the app does not automatically load a root
`.env` file. A local Go/Vue process alone does not emulate the Asterisk media
bridge or a configured Twilio account. Database migrations run at startup.

GNU Make wraps these commands; `make help` lists supported targets.
The isolated `scripts/pbx-integration-test.py` requires a Linux Docker host,
the staging PBX image, trusted test certificates and the paths documented at
its top. It uses synthetic accounts and a fake backend, with no paid Twilio
calls or messages. Production acceptance calls/SMS remain manual.

For an artifact-free archive and safe cache cleanup, follow
[source-only backups](SOURCE_BACKUP.md). Do not rename the production Compose
files or remove embedded migrations while rearranging source.

For read-only deployment checks, run
`python3 scripts/deployment-smoke.py --http https://sip.example.com --tls-host sip.example.com`.
This checks health, static assets, route fallbacks, anonymous API rejection and
the trusted PBX TLS certificate. It does not place calls or send messages.
