# Leadomi SIP user guide

## Console

- **Dashboard**: backend/PBX/provider configuration health, last host backup,
  on-demand database backups and recent administrator changes.
- **SIP Users**: create a phone identity, explicitly review and assign an owned
  number, download setup details, edit the name, reset a phone password or disable.
- **Phone Numbers**: read-only live Twilio inventory and local assignments.
- **Call Routing**: local incoming routing rules. Number webhook changes are
  handled only by confirmed assignment, not by browsing this screen.
- **Call History / Activity Log**: paginated call/SMS records with who, direction,
  from/to, status and time. Saved actor labels stay stable on later reassignment.
- **Messages**: view all business numbers or filter one number, load older
  messages, and select a sending number for replies. Start a conversation using
  an international `+number`. Status is submission/delivery status, not a read
  receipt. Refresh reloads locally stored history.
- **Voicemails**: available recordings, duration and provider transcripts.
- **Settings**: your administrator password and outgoing usage policies.
- **Administrators**: trusted web accounts, separate from phone users.

All web administrators can see business-wide history. Phone users only need
their phone account. Times in Activity Log use the viewing device's timezone.
Older records explicitly say when actor information was not captured.

## Delete history

Administrators can use **Delete** in Messages or **Delete record** in Call History
and Activity Log. For bulk deletion, select individual records or use **Select all
on this page** / **Select all loaded messages**, then click **Delete selected**.
Load older messages first if you want to include them. Activity Log can select
message records across conversations; Call History contains calls only.

To delete whole chats with different numbers, use **Delete conversation** beside
a conversation, or tick several conversation checkboxes and click **Delete
selected conversations**. **Select all conversations** includes all chats in the
current business-number filter. The app loads every older page before showing
confirmation, so you do not need to open each chat or load its messages manually.
Choose **All business numbers** to include those chats across your business
numbers. Only the prepared messages are deleted; newly arriving messages remain.

Review the selected records, choose **Dashboard only** or **Dashboard and Twilio**,
tick the confirmation box, and submit. Bulk deletion shows progress, processes
each selected record once, and reports failures individually. Keep the page open
until processing finishes. Retrying processes only the records that failed.

Dashboard-only deletion keeps the Twilio record, which can be imported again.
Deleting from both removes that specific Twilio resource and its dashboard row.
Active records are blocked. If Twilio verification or deletion fails, local
history is kept. Administrator deletions are recorded in the audit log.

Twilio message deletion also removes associated media; it does not recall an SMS
from a recipient's phone. Call-record deletion keeps recordings, transcriptions,
and other call legs. Existing backups remain unchanged. See Twilio's
[message deletion](https://www.twilio.com/docs/messaging/api/message-resource#delete-a-message-resource)
and [call deletion](https://www.twilio.com/docs/voice/api/call-resource#delete-a-call)
documentation for provider retention details.

## Android phone

Use the downloaded account setup file. In Linphone, configure the SIP identity,
registrar **and** outbound proxy, TLS on 5061, SRTP, and CPIM disabled for basic
conversations. See [Softphones](TWILIO_SOFTPHONES.md) for exact fields and tests.
There is one registered phone per SIP user. A second phone needs a separate SIP
user and explicitly assigned owned number. SMS is bridged to Twilio's HTTP API;
ordinary phone carrier SMS is a separate service.

Calls use `+country-code-number`; messages use `sip:+number@your-sip-hostname`.
Allow Android background activity and reconnect Linphone after service restarts.
Offline incoming SMS is queued, but background delivery and ringing depend on
the phone remaining reachable. Softphone read receipts, typing indicators,
browser calling, conferencing and transfer controls are not supported here.

## Troubleshooting

| Symptom | Check |
|---|---|
| Account connection error | Hostname, TLS5061, trusted certificate, username/password and both proxy fields |
| Incompatible media | Enable SRTP; allow G.711/PCMU/PCMA where the client exposes codec settings |
| 484 / international-number prompt | Both proxy fields; use an actual leading `+` and full country code |
| One-way or no audio | UDP10000–10100, public IP advertisement, Android network/NAT |
| Message spinner or failure | Disable CPIM; use `sip:+number@host`; inspect status and username in Activity Log |
| Incoming SMS missing | Number-level webhook, Messaging Service inbound handler, assigned user's registration |
| Cannot assign number | Active local owner, failed provider connection check, service override, stale review |
| Refresh logs out | Valid HTTPS cookie/session; password reset intentionally revokes sessions |

Check **Dashboard** and provider status before changing working credentials.
Do not delete volumes or re-run first setup as a troubleshooting shortcut.
