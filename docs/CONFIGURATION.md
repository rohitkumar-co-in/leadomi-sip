# Leadomi SIP configuration

The existing `GOSIP_*` environment names and `gosip` SIP realm remain stable.
Do not rename production volumes, databases, or secret variable names when
rebranding. Use the same public hostname for HTTPS and phone setup.

## Required business deployment variables

| Variable | Meaning |
|---|---|
| `TWILIO_ACCOUNT_SID` | Twilio `AC…` account identifier |
| `TWILIO_AUTH_TOKEN` | API/webhook secret; Go backend only |
| `TWILIO_SIP_DOMAIN` | Dedicated hostname such as `company.sip.twilio.com` |
| `GOSIP_PUBLIC_URL` | Public HTTPS origin, e.g. `https://sip.example.com` |
| `GOSIP_CORS_ORIGINS` | That same trusted HTTPS origin |
| `GOSIP_PBX_URL` | Private Docker URL `http://pbx:8088` |
| `GOSIP_PBX_SECRET` | Random shared backend/PBX HTTP secret |
| `GOSIP_PBX_SIP_DOMAIN` | Public phone hostname, e.g. `sip.example.com` |
| `PBX_DOMAIN` | Same public phone hostname |
| `PBX_PUBLIC_IP` | Static public IPv4 for SIP/RTP NAT advertisement |
| `GOSIP_PBX_TRUNK_USER` | Private incoming voice trunk username |
| `GOSIP_PBX_TRUNK_PASSWORD` | Random incoming trunk password |
| `PBX_TWILIO_USER` | Dedicated bootstrap Twilio SIP credential username |
| `PBX_TWILIO_PASSWORD` | Bootstrap Twilio SIP credential password |
| `GOSIP_OUTBOUND_CALLER_ID` | Owned E.164 fallback number; no auto assignment |
| `GOSIP_VOLUME_NAME` | Existing Go data volume name, Coolify compose only |
| `TZ` | Operational timezone, e.g. `Asia/Kolkata` |

Generate secrets privately, for example with `openssl rand -hex 24`.
PBX usernames use letters, digits, underscores, or hyphens. Trunk passwords
also support common punctuation (`@#!$%&*()+.,=:-`) up to 128 characters;
spaces, semicolons, quotes, backslashes and line breaks are rejected to keep
rendered Asterisk configuration safe. Keep incoming trunk and shared HTTP
secrets distinct. The custom **phone** password accepts printable ASCII without
spaces (12–72 characters), because only its digest hash enters Asterisk config.

`PBX_TWILIO_DEVICE_PASSWORDS` is an optional JSON map for preserving credentials
from an existing installation. Leave it empty on a fresh installation. Newly
provisioned credentials use the private `credentials.json` volume file instead.
Secondary Account SIDs and Auth Tokens are saved in
`/app/data/twilio-accounts.json`, outside SQLite. Do not define
`GOSIP_PBX_UNUSED_API_CREDENTIAL`: its empty alias keeps Twilio API credentials
out of the PBX environment when Coolify injects app variables.

## Fixed compose settings

The backend listens privately on 8080 and persists `/app/data/gosip.db` and
secondary Twilio account metadata/secrets.
The PBX listens publicly on TLS TCP 5061 and media UDP 10000–10100; its private
health/credential API listens on 8088. Certificates mount from
`/data/gosip-pbx/certs`. PBX credentials, offline SMS delivery state and backup
markers persist under `/var/lib/gosip-pbx`. `PBX_TWILIO_NETWORKS` identifies
provider signaling networks in the compose file; review against current
Twilio documentation before changing provider regions.

The standalone Go SIP/TLS/WSS variables in legacy code do not configure the
supported Asterisk service. Do not expose its 5060 or enable browser WebRTC
expecting it to manage this deployment's audio.

## Database metadata and UI policies

`pbx_twilio_domain_sid` and `pbx_twilio_credential_list_sid` are non-secret
dedicated Twilio resource identifiers; see [Installation](INSTALLATION.md).
`pbx_device_numbers` is a compatibility map maintained by provisioning; do not
hand-edit a working system's assignments. `sip_accounts` journals account state.

**Settings** stores allowed outgoing E.164 prefixes and the SMS-per-minute
limit. Empty prefixes allow all destinations; default SMS limit is 20 per
number per minute. Asterisk limits two concurrent outgoing calls per SIP user.
These are usage controls, not monetary caps. Configure spending alerts in
Twilio separately. **Administrators** controls web access; SIP users are
separate phone identities and have no web-console login.

The primary Twilio account uses runtime variables (with legacy `/app/data/.env`
fallback on older installations); secondary account tokens use
`/app/data/twilio-accounts.json`. Neither token type is stored in SQLite. The
PBX receives empty Twilio API environment aliases, but its read-only mount of
the Go data volume makes secondary-token files readable to the PBX container's
root process. See [Credentials](TWILIO_CREDENTIALS.md) for this trust boundary.
