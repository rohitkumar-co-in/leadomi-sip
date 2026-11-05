# Leadomi SIP credential storage

| Credential | Storage / role |
|---|---|
| Twilio Account SID + Auth Token | Go backend runtime environment; API and signed webhook validation |
| Dedicated Twilio SIP password | Twilio Credential List plus root-only PBX state; outbound voice authentication |
| Phone SIP password | Shown once on create/reset; only digest hash in SQLite/Asterisk |
| Administrator password | bcrypt hash in SQLite |
| PBX shared HTTP secret | Runtime environment on backend and PBX |
| Incoming voice trunk password | Runtime environment; Twilio→PBX SIP authentication |
| TLS private key | Root-owned host certificate directory mounted read-only |

Prefer Coolify runtime variables for the Twilio API credentials. They override
legacy fallback `/app/data/.env` (0600) and are not copied into SQLite. The PBX
compose service deliberately receives empty API-credential aliases.
SIP Domain/Credential List SIDs are non-secret metadata stored in config.

Creating a phone user provisions a separate Twilio SIP credential only after
number review and confirmation. It never reuses the phone password as a
Twilio API token. Editing a ready user with the same number does not rotate
Twilio credentials; an explicitly requested phone password reset changes only
the local digest. Keep downloads private and delete unneeded plaintext copies.

**Disable** revokes the dedicated user's Twilio credential, blocks local new
calls/SMS, and retains owned numbers/history. If revocation fails, retry it.
Rotate API credentials through Coolify and redeploy, then verify signed
webhooks. Coordinate any incoming trunk or dedicated SIP password rotation
across both endpoints; avoid resetting a working phone to fix unrelated routing.

Root access to the VPS or Coolify can read deployment secrets. HTTPS protects
web transport; SIP uses TLS and SRTP. A domain name, including sslip.io, does not
by itself make a credential private. The server permissions and admin access
boundary determine who can read it.
