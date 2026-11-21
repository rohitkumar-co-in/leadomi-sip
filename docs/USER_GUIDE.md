# Leadomi SIP user guide

## Console

- **Dashboard**: phone, phone server, AI and incoming-call status; the
  read-only **Setup check** (Twilio, webhooks, certificate, call audio address,
  Linphone, browser dialer) with a fix for each red item; backups; recent changes.
- **Dialer**: call from any browser (keypad, mute, DTMF, recent calls with call
  back / text, microphone and speaker choice). Incoming calls ring the browser
  and Linphone together; an Answer/Decline pop-up shows on every page.
- **Phone Setup**: Linphone settings with copy buttons, the automatic setup
  link, and Windows/Android/iPhone steps.
- **Phone Numbers**: Twilio numbers; **Use for phone** chooses the business number.
- **Activity Log**: calls and texts with status, who and when.
- **Messages**: conversations; start one with a `+number`.
- **Simulator**: fictional `+447700900xxx` callers to test text-back and AI
  without Twilio credits (can be turned off in Settings).
- **Voicemails**: recordings and transcripts.
- **Settings**: **Features status** (n8n, text-back, Demo mode, AI replies,
  Simulator, browser dialer, Linphone), switches for Demo mode, AI replies and
  Simulator, your password, and usage controls (allowed country prefixes, SMS
  rate).
- **Administrators**: console accounts.

Times use the viewing device's timezone.

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
registrar **and** outbound proxy, TLS on the port shown in the downloaded setup
file (5061 by default), SRTP, and CPIM disabled for basic
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

See [Troubleshooting](TROUBLESHOOTING.md) for every known problem and fix.

| Symptom | Check |
|---|---|
| Account connection error | Hostname, configured TLS port (5061 by default), trusted certificate, username/password and both proxy fields |
| Incompatible media | Enable SRTP; allow G.711/PCMU/PCMA where the client exposes codec settings |
| 484 / international-number prompt | Both proxy fields; use an actual leading `+` and full country code |
| One-way or no audio | Configured RTP range (10000–10100 by default), public IP advertisement, Android network/NAT |
| Message spinner or failure | Disable CPIM; use `sip:+number@host`; inspect status and username in Activity Log |
| Incoming SMS missing | Number-level webhook, Messaging Service inbound handler, assigned user's registration |
| Cannot assign number | Active local owner, failed provider connection check, service override, stale review |
| Refresh logs out | Valid HTTPS cookie/session; password reset intentionally revokes sessions |

Check **Dashboard** and provider status before changing working credentials.
Do not delete volumes or re-run first setup as a troubleshooting shortcut.
