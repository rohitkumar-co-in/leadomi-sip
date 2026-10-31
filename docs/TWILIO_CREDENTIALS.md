# Leadomi SIP credential storage

| Credential | Storage and purpose |
|---|---|
| Primary Twilio Account SID and Auth Token | Go backend runtime environment; Twilio API calls and signed webhook validation |
| Secondary Twilio Account Auth Tokens | `twilio-accounts.json` in the Go data volume; secondary account API calls and signed webhook validation |
| Dedicated Twilio SIP password per phone user | Twilio Credential List and private PBX `credentials.json`; outbound voice authentication to the assigned account's SIP Domain |
| Phone SIP password | Returned once on create/reset; only its digest is stored in SQLite/Asterisk; authenticates Linphone to Leadomi |
| Administrator password | bcrypt hash in SQLite |
| PBX shared HTTP secret | Runtime environment on backend and PBX |
| Incoming voice trunk password | Runtime environment; authenticates Twilio-to-PBX SIP traffic |
| TLS private key | Root-owned host certificate directory mounted read-only |

Twilio Account Auth Tokens and phone passwords are different credentials. A
secondary account token belongs only in Leadomi's settings form. Never place an
Auth Token, Twilio SIP password, or phone password in a phone's Domain or
Username field, a public log, or source control.

Creating or assigning a phone provisions a dedicated outbound SIP credential
in the selected account's Credential List and stores the matching password in
private PBX state. The phone's Leadomi login password remains separate. Editing
a ready user with the same number preserves provider credentials; resetting the
phone password changes only the local SIP digest.

## Recovery when outbound SIP authentication drifts

Symptoms can include the PBX reaching the secondary SIP Domain over TLS while
Twilio creates no call record, or an Asterisk outbound leg returning
`CHANUNAVAIL` with Q.850 cause 21. These signs point to a rejected remote SIP
leg; they do not by themselves identify the exact Twilio error.

For an already-assigned phone, the normal same-number edit path does not
resynchronize its Twilio credential. A trusted maintainer must:

1. Identify the SIP user's assigned Twilio Account SID, Credential List, and
   dedicated credential SID.
2. Generate a new strong outbound SIP password and update that credential in
   the correct Twilio Credential List using that account's Auth Token.
3. Atomically update the matching username in the PBX's private
   `credentials.json` to the same password, keep the file mode at `0600`, and
   allow the PBX configuration loop to reload it.
4. Make an authorized short outbound test and verify the secondary Voice URL
   returns HTTP 200 and a call record appears under the assigned Twilio account.

Do not change the Linphone password to repair this server-to-Twilio credential
pair. Do not print either password or the Auth Token while doing the repair.

## Storage boundaries

The primary API credentials are supplied to the Go backend as runtime
environment values. Secondary account tokens are kept outside SQLite in
`twilio-accounts.json`, with restrictive file permissions. The Coolify Compose
deployment mounts the Go data volume read-only into the PBX so the bridge can
read `gosip.db`. That mount is not a confidentiality boundary: the PBX
container's root process can read other files in the volume, including the
secondary-token file. Treat both containers and the host as trusted. The PBX
code does not use the account token, but the current shared-volume layout makes
it accessible to the container.

Backups contain secondary account tokens when configured. Encrypt and tightly
restrict backup copies. Root access to the VPS or Coolify can read deployment
secrets. A read-only mount protects against writes; it does not prevent reads.

**Disable** revokes a user's dedicated Twilio credential and blocks new local
calls/messages while retaining history and the number. If remote revocation
fails, retry Disable. Rotate API credentials through Coolify and reconnect the
secondary account in Leadomi, then verify signed webhooks. Avoid resetting a
working phone to diagnose unrelated routing problems.
