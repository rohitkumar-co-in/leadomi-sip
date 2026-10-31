# Leadomi SIP architecture

Supported product topology: Vue administrator console → Go/Chi HTTP backend →
SQLite and one or more Twilio APIs; Android Linphone → Asterisk PJSIP TLS/SRTP
→ the Twilio account that owns the phone user's assigned number.
SIP MESSAGE goes through authenticated per-device Asterisk contexts and private
HTTP to the backend. Signed inbound SMS is persisted and delivered or queued by
the PBX bridge. Twilio status callbacks update persistent records.

`internal/api/business*.go` implements read-only inventory/review, explicit
provisioning, policies, activity and audit. `sip_accounts` journals provisioning;
`pbx_device_numbers` is retained compatibility metadata. `activity_actors`
captures attribution at record creation, with explicit historical unknowns.
The primary API token is a backend environment value; secondary account tokens
are held in `twilio-accounts.json` outside SQLite. Phone hashes are SQLite; each
account's outbound SIP credential is stored in the corresponding Twilio
Credential List and private PBX state. Provider and local DB writes are not
atomic together; errors are visible and retries require review.

`deploy/asterisk/bridge.py` renders endpoints and isolated MESSAGE/call contexts
from read-only Go DB data. The assigned account selects that user's Twilio SIP
Domain and outbound credential. The bridge preserves TLS5061/SRTP, one contact
per phone user, per-user caller ID, private credential storage and offline SMS
delivery state.
Existing route edits and phone password resets do not change number webhooks.
Only confirmed number assignment changes Twilio callback/application links.

In the Coolify Compose layout, PBX mounts the Go data volume read-only to access
SQLite. This also makes other files in that volume readable by the PBX
container's root process, including secondary Auth Tokens. The bridge does not
use those tokens, but the mount is not a confidentiality boundary.

Go module identity is `github.com/rohitkumar-co-in/leadomi-sip`. Legacy standalone SIP,
vendor provisioning, transfer/MOH and WebRTC source remains for compatibility
or development and is not advertised as the supported Asterisk product.
Runtime paths, environment keys, volume identities and SIP realm remain stable.

See [configuration](CONFIGURATION.md),
[API](API.md), [operations](BUSINESS_OPERATIONS.md).
