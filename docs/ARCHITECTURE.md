# Leadomi SIP architecture

Supported product topology: Vue administrator console → Go/Chi HTTP backend →
SQLite/Twilio APIs; Android Linphone → Asterisk PJSIP TLS/SRTP → Twilio Voice.
SIP MESSAGE goes through authenticated per-device Asterisk contexts and private
HTTP to the backend. Signed inbound SMS is persisted and delivered or queued by
the PBX bridge. Twilio status callbacks update persistent records.

`internal/api/business*.go` implements read-only inventory/review, explicit
provisioning, policies, activity and audit. `sip_accounts` journals provisioning;
`pbx_device_numbers` is retained compatibility metadata. `activity_actors`
captures attribution at record creation, with explicit historical unknowns.
The dedicated API token stays in backend environment. Phone hashes are SQLite;
outbound SIP credentials are private PBX state. Provider and local DB writes are
not atomic together; errors are visible and retries require review.

`deploy/asterisk/bridge.py` renders endpoints and isolated MESSAGE/call contexts
from read-only Go DB data. It preserves TLS5061/SRTP, one contact per phone user,
per-user caller ID, private credential storage and offline SMS delivery state.
Existing route edits and phone password resets do not change number webhooks.
Only confirmed number assignment changes Twilio callback/application links.

Go module identity is `github.com/rohitkumar-co-in/leadomi-sip`. Legacy standalone SIP,
vendor provisioning, transfer/MOH and WebRTC source remains for compatibility
or development and is not advertised as the supported Asterisk product.
Runtime paths, environment keys, volume identities and SIP realm remain stable.

See [configuration](CONFIGURATION.md),
[API](API.md), [operations](BUSINESS_OPERATIONS.md).
