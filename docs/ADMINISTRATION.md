# Leadomi SIP administrator guide

The console is for trusted administrators. It exposes business-wide call/SMS
history and configuration. Phone users have separate SIP identities.

## Create a phone user

1. Open **SIP Users → Add SIP user**. Enter the person's name and custom SIP
   username. Usernames start with a letter and use 3–64 letters, digits,
   underscores or hyphens. Internal names (`global`, `tls`, `deny`, `twilio-*`)
   are reserved. Existing Twilio usernames are not taken over.
2. Select a number already owned by your Twilio account with both Voice and SMS
   capability. A number assigned to another active local user is unavailable.
   Leadomi SIP does not buy, port, release or automatically assign numbers.
3. Enter an optional custom phone password (12–72 printable ASCII characters,
   no spaces), or leave it empty to generate a strong password.
4. Wait for **Review number connections**. Read the existing voice/SMS/status
   webhook URLs, TwiML application links, Messaging Service membership, and
   local call routes. Other systems' outbound-only use cannot be detected.
5. Only if you intend to move the selected number, tick the confirmation and
   choose **Assign and provision**. If the configuration changed since review,
   saving is rejected; reopen the form and review again.
6. The server creates the dedicated Twilio SIP credential, stores a separate
   private PBX password, sets the selected number's call/SMS/status webhooks,
   clears its TwiML application links, creates a local default route, and
   assigns caller ID/SMS sender. Download the phone setup file immediately.

**Assignment can disconnect other inbound integrations.** Do not confirm when
you need to keep those connections. Messaging Services with their own inbound
handler block assignment; Leadomi SIP does not change service settings. Resolve
that service manually in Twilio only after deciding which integration should
receive messages. A service using number-level inbound webhooks is retained.

Inventory, startup, login and page refresh are read-only toward Twilio. Existing
working users are imported without resetting passwords or number callbacks.

## Excluded numbers

Use **Excluded numbers** to protect a number belonging to another integration.
Enter its international `+number` and an optional reason. The list is persistent
and does not change current calls, SMS, credentials or callbacks. It may also
contain numbers not currently owned, to protect future imports.

Protected numbers are unavailable for assignment. The API also blocks edits,
password resets, disabling/deleting linked SIP users and local route changes.
There is no force bypass. Global routing changes are blocked while exclusions
exist because they could affect protected numbers. Read-only inventory and
connection checks remain available. Remove protection explicitly before making
changes; removal itself does not reconfigure anything. Direct Twilio or host
administrator changes are outside this console's protection.

## Edit, reset, reassign, disable, delete

**Edit / reset password** on a ready enabled user with the same number changes
only the display name and an explicitly requested phone password. It preserves
Twilio credentials, webhooks, Messaging Services, and local call routes. SIP
username is fixed after creation. Resetting a phone password requires updating
the phone; it does not reset the separate Twilio credential.

Selecting a different number or retrying incomplete provisioning requires
another connection review. The old number retains its Twilio callbacks but its
local routes are removed; it no longer rings the reassigned phone. The new
number's local routes are replaced with the assigned-user default.

**Disable** blocks new calls/messages locally and revokes the dedicated Twilio
credential. It retains history and the owned number; a call already in progress
may continue. If remote revocation fails, retry Disable. A disabled user's
number can later be explicitly assigned to a replacement user.

After disabling, **Delete** removes the SIP user from the active list and clears
its local assignment and phone password hash. Deletion verifies remote
credential revocation first; provider failure pauses deletion for retry. It
retains the owned Twilio number, its callbacks, historical user ID and call/SMS
history. This is an archived local record, so its old username cannot be reused.
It does not immediately terminate a call already in progress.

An incomplete new account is marked `error` and cannot send calls/SMS until
**Configure / retry** completes. External Twilio writes and SQLite cannot share
a transaction: a failure may leave partial provider changes. Review before
retrying and inspect the audit log. Do not repeatedly reset working accounts to
diagnose unrelated network problems.

## Web administrators and first setup

A fresh server's `/setup` creates the first administrator, using an email and
12–72 character password. There is no default password. Restrict public access
until this step is complete. Afterward **Administrators → Add User** creates
additional administrator accounts. Only trusted people should receive access.
**Settings** changes your own password. Password changes/reset revoke existing
web sessions; sign in again. Removing another administrator does not remove
their unrelated SIP identity.

## History, policies, and routing

**Activity Log** combines calls and SMS: time, direction, from/to, account,
status, duration or SMS body. **Call History** uses the same log filtered to
calls. New outbound phone SMS records capture the authenticated SIP username;
web SMS captures the administrator email. Incoming records capture the assigned
receiving account, not proof of who answered/read. Historical records without
an actor remain explicitly unknown; reassignment does not rewrite saved actors.

**Messages** selects a business number and conversation, and sends SMS through
that number. **Call Routing** changes local incoming rules; it does not change
Twilio number webhooks. **Voicemails** exposes recorded voicemail and any
available provider transcript. **Dashboard** shows health, backup status, and
recent configuration audit entries. In **Settings**, choose allowed outgoing
country prefixes and an SMS limit. See [Operations](BUSINESS_OPERATIONS.md).
