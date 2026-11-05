# Leadomi SIP business operations

The supported deployment uses Asterisk for SIP/media, the Go backend for the
administrator console and SMS bridge, and Twilio for carrier calls/SMS. Each
SIP user has one owned assigned number and one registered phone.

## Daily operations

Check **Dashboard** for PBX health, provider credentials configured, last daily
backup and recent configuration changes. Review **Activity Log** for failed
calls or messages with direction, from/to, actor and timestamp. Incoming actor
labels identify the receiving account; they are not read receipts or proof of
who answered. Historical records without captured actor data stay unknown.

Use **SIP Users** for [reviewed assignments](ADMINISTRATION.md), phone-password
reset, disabling or deleting a disabled user. Ready users edited with the same number retain their
Twilio credentials and number routes. Inventory/startup/login do not configure
numbers. Never use re-provisioning as a substitute for reconnecting a phone.

Use **Excluded numbers** to persistently protect another system's number from
assignment and configuration changes. Adding protection preserves existing
traffic; linked SIP edits, disabling/deletion and routing changes are blocked
until the exclusion is explicitly removed. There is no force override.

## Usage and access controls

Only trusted web administrators can access business-wide history. Phone users
do not need a web account. Administrator password changes revoke old sessions.
Twilio API credentials stay in backend runtime environment.

**Settings** restricts outgoing E.164 prefixes (`+44`, `+91`, etc.) and the
SMS-per-minute submission limit (default20 per number). Empty prefixes allow
all destinations. Asterisk permits two concurrent outgoing calls per SIP user.
Configure Twilio spend/usage alerts separately: these controls are not a
monetary budget cap. Disabled users retain history and their Twilio number;
an already connected call can continue.

## Updates and incidents

Build images use Node24, Go1.26 and Alpine3.23; Asterisk uses Ubuntu24.04 package
updates. Keep dependency/OS updates tested. Use the same app and volumes for
updates, back up first, and verify existing callback mappings read-only after
restart. Reopen Linphone and perform manual paid tests when appropriate.

If provisioning fails, inspect its visible state and audit before retrying.
Twilio and SQLite cannot share a transaction, so partial provider state may
exist. Assignment never edits a Messaging Service inbound policy automatically;
an overriding service blocks it. Failed credential revocation requires a
Disable retry. Do not expose private SIP traces or container environments in
public logs because they can contain secrets.

## Recovery and limitations

Daily root-only [backups](BACKUP.md) keep30days and record success in the
dashboard. Verify an isolated restore periodically. Off-server storage is
currently unconfigured and must be arranged separately. One VPS is not a
redundant phone service. Retain Coolify environment and TLS/DNS recovery access.

Android background restrictions can delay ringing/messages. Offline SMS is
queued; delivery acceptance is not a read receipt. MMS on mobile SIP clients,
browser WebRTC, conferences and transfer controls are not supported here.
No automatic paid call or SMS tests are needed for deployment checks.
