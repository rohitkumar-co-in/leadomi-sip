# Connect another Twilio account

Leadomi can connect a secondary Twilio account while leaving its phone number in
that account. Each SIP user and assigned number uses one Twilio account. The
number must support both Voice and SMS for the current business provisioning
flow.

## Prepare the second Twilio account

Sign in to the account that owns the number. Create a dedicated Programmable
Voice SIP Domain and a dedicated Credential List. Enable secure calling and SIP
registration, then attach that Credential List to both **Calls** and
**Registrations** authentication on the SIP Domain. Set the SIP Domain's outgoing
Voice URL to:

```text
https://YOUR-LEADOMI-HOST/api/webhooks/voice/outgoing
```

Use POST. Replace `YOUR-LEADOMI-HOST` with the public HTTPS host configured for
Leadomi. Do not reuse a SIP Domain that serves another application.

Copy the account's Account SID, SIP Domain SID (`SD…`), and Credential List SID
(`CL…`). Keep the Auth Token private.

## Connect it in Leadomi

1. Deploy the updated Leadomi version so database migrations run and the new
   **Settings → Additional Twilio accounts** form appears.
2. Enter the Account SID, Auth Token, SIP Domain SID, and Credential List SID.
   Leadomi checks the credentials, secure domain, outgoing Voice URL, and both
   Credential List mappings before saving.
3. Open **SIP Users → Add SIP user**. The second account's eligible numbers
   appear in the number selector with their account SID.
4. Select the number, inspect its current Voice, SMS, Messaging Service, and
   local routing connections, then confirm assignment if those connections may
   be replaced.
5. Download the one-time phone setup file and configure Linphone. Test inbound
   and outbound calls and SMS before relying on the number.

Leadomi stores the secondary Auth Token in `twilio-accounts.json` in its private
data directory with owner-only file permissions. The token is not stored in
SQLite or returned to the browser. Back up the data volume securely; the backup
contains this credential file.

If the account is connected but the number cannot be provisioned, verify that
its SIP Domain still has the expected secure setting, Voice URL, and Credential
List mappings. The Settings form can be submitted again to rotate a token and
refresh the SIP resource metadata.
