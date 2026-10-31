# Connect another Twilio account

Leadomi can connect a secondary Twilio account without moving its phone number
to the primary account. Each assigned SIP user and number is routed through the
Twilio account that owns that number. The phone still registers to Leadomi's SIP
server; it does not register directly to either Twilio account.

## Prepare the Twilio account

In the account that owns the number, create a dedicated Programmable Voice SIP
Domain and Credential List. Enable secure calling and SIP registration, then
attach the Credential List to both **Calls** and **Registrations**.

Set the SIP Domain's **A call comes in** Voice URL to
`https://YOUR-LEADOMI-HOST/api/webhooks/voice/outgoing` with method POST. Set
**Call status changes** to
`https://YOUR-LEADOMI-HOST/api/webhooks/voice/status` with method POST. If Twilio
requires a **Primary handler fails** URL, it must return valid TwiML; do not use
the status callback URL for that field. Replace the host with Leadomi's public
HTTPS hostname. Keep this SIP Domain dedicated to Leadomi.

Record the Twilio Account SID (`AC…`), SIP Domain SID (`SD…`), and Credential
List SID (`CL…`). Keep the Auth Token private.

## Connect and assign a number

1. Deploy the Leadomi version with **Settings → Additional Twilio accounts**.
2. Enter the Account SID, Auth Token, SIP Domain SID, and Credential List SID.
   Leadomi validates the account credentials, secure SIP Domain, outgoing Voice
   URL, and both Credential List mappings before saving.
3. Open **SIP Users → Add SIP user**. Eligible owned numbers from connected
   accounts appear in the number selector with their account SID.
4. Review the selected number's Voice, SMS, Messaging Service, and local route
   connections. Confirm assignment only if those connections may be replaced.
5. Download the generated phone setup file and configure Linphone with the
   Leadomi registrar and proxy from that file.

Provisioning creates or updates a dedicated Twilio SIP credential for the user,
separate from the phone's Leadomi login password. Leadomi stores the matching
outbound credential in private PBX state. The SIP user and number are then
routed through that account's SIP Domain and signed Voice URL.

The secondary Auth Token is stored in `twilio-accounts.json` in the Go data
volume with restrictive file permissions. It is not stored in SQLite or sent to
the browser. The backup archive includes this file; protect and encrypt any
off-server copy. In the Coolify Compose layout, the Go data volume is also
mounted read-only into the PBX for database access. Read-only does not prevent
the PBX container's root process from reading other files in that volume; treat
the PBX container and host as trusted infrastructure.

## Test the call path

For an authorized short outbound call, verify the whole chain:

1. The PBX establishes TLS to the assigned Twilio SIP Domain.
2. The secondary account's `/api/webhooks/voice/outgoing` request reaches
   Leadomi and returns HTTP 200.
3. Twilio creates a call record in the same account that owns the assigned
   number, and the destination rings.

An app health check or TLS connection alone does not prove the call path. Check
the account associated with the SIP user's assigned number when reviewing
Twilio logs.

## Troubleshoot outbound calls

| Symptom | What to check |
|---|---|
| Linphone says Unauthorized | Check the phone username and phone password against the latest setup file, and keep the registrar/proxy on Leadomi. This is separate from Twilio's outbound credential. |
| PBX reaches Twilio over TLS, but Twilio has no call record | Check the secondary account's Calls Credential List mapping and whether its SIP credential matches Leadomi's private PBX credential. A TLS connection does not prove SIP authentication succeeded. |
| PBX reports `CHANUNAVAIL` with Q.850 cause 21 | The remote SIP leg rejected the call. Check Twilio's SIP authentication/account status and the device's dedicated credential before changing the phone password. |
| `484 Address Incomplete` / cause 28 | Confirm the destination is the full international number with a leading `+`. If it is already valid and Twilio has no call record, inspect the PBX-to-Twilio leg and credential state; do not assume a country-permission issue from this response alone. |
| Leadomi receives the Voice URL request but the call does not ring | Check that the request is for the correct account, the assigned caller ID belongs to it, the destination is allowed, and Twilio logs contain the resulting child call. |

The Linphone password is not the Twilio SIP Credential List password. A phone
password reset does not repair a mismatch between Twilio's Credential List and
the PBX's `credentials.json`. For an assigned user with confirmed credential
drift, a trusted maintainer must generate a new dedicated outbound password,
update that user's credential in the correct Twilio Credential List, atomically
write the same value to private PBX state with mode `0600`, and wait for the PBX
configuration reload. Never print either password or the Auth Token. The normal
same-number edit path preserves provider credentials and is not a credential
resynchronization action.
