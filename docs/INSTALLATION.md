# Fresh Leadomi SIP installation

For the quickest correct path, use the
[complete setup checklist](SETUP_CHECKLIST.md). This page gives background.

Use this guide for a new server. For an existing server, follow the preservation
and update procedure in [Coolify](COOLIFY_DEPLOYMENT.md). Never empty an existing
data volume or run initial setup again to change branding or configuration.

## Prerequisites

Use a maintained Ubuntu VPS with a static public IP (AWS: **Elastic IP**;
a changing IP breaks call audio), Docker/Compose v2, DNS
control, and a Twilio account with owned voice-and-SMS-capable numbers. Start
with at least two CPU cores, 2 GB RAM and 10 GB free disk; adjust to actual load.
Builds require additional headroom. Hostname examples below use
`sip.example.com`; substitute your hostname consistently.

Configure its DNS A record to the VPS. Open HTTPS TCP 443, HTTP TCP 80 when
needed for certificate renewal, SIP TLS TCP 5061, and RTP UDP 10000–10100 by
default. Run one Leadomi SIP installation per VPS.
Restrict SSH to your management network. Do not expose the Go backend's 5060,
PBX HTTP 8088, AMI, database, or unauthenticated management ports publicly.

## Dedicated Twilio resources

1. In Twilio Console, create a dedicated **Programmable Voice SIP Domain**.
   This uses SIP Domain credentials, not the legacy Elastic SIP Trunk editor.
2. Enable **Secure media** and **SIP registration** (both required). Create a dedicated Credential
   List and attach it to both **Calls** and **Registrations** authentication.
3. Add a bootstrap SIP credential to that list with a unique username and a
   generated alphanumeric password. Put these in `PBX_TWILIO_USER` and
   `PBX_TWILIO_PASSWORD`. `PBX_TWILIO_USER` is also the Linphone username.
4. Set the SIP domain's outgoing Voice URL to
   `https://sip.example.com/api/webhooks/voice/outgoing`, method POST. This
   domain must be dedicated to this deployment: do not repoint a domain used
   by an unrelated working system. Twilio also requires a **Primary handler
   fails** URL: use a TwiML Bin that says the line is unavailable.
5. Record the Domain SID (`SD…`) and Credential List SID (`CL…`) and set them
   as `TWILIO_SIP_DOMAIN_SID` and `TWILIO_CREDENTIAL_LIST_SID`.

Do **not** change any phone number's callbacks at this stage. Number inventory
is read-only; assigning a number later requires review and confirmation.
Buying or porting numbers remains an explicit action in Twilio Console.

## Environment and TLS

Clone this repository and configure the variables in
[Configuration](CONFIGURATION.md). Coolify runtime secrets are recommended.
For direct Docker Compose, copy `.env.example` to `.env`, populate it, and
restrict it to mode 0600. Never commit that file.

With Coolify, the PBX reuses the proxy's certificate automatically; see
[Coolify deployment](COOLIFY_DEPLOYMENT.md). For direct Compose, provide a
publicly trusted certificate for the SIP hostname in the directory
configured by `PBX_CERTS_HOST_PATH` (default `/data/gosip-pbx/certs`), named
`fullchain.pem` and `privkey.pem`. The root-owned private
key should be mode 0600. The PBX mounts this directory read-only. Your HTTPS
proxy must also present a trusted certificate. Install a renewal/export hook
so the PBX certificate stays current; a HTTPS proxy renewal alone does not
guarantee that the mounted SIP files were refreshed.

For Coolify, follow its deployment guide. For direct Compose:

```sh
git clone https://github.com/rohitkumar-co-in/leadomi-sip.git
cd leadomi-sip
cp .env.example .env
# Edit .env privately before continuing.
chmod 600 .env
docker compose --project-name leadomi-sip -f docker-compose.production.yml up -d --build
```

The direct deployment binds web HTTP only to `127.0.0.1:8080`. Put a HTTPS
reverse proxy in front of it, passing Host and `X-Forwarded-Proto: https`.
For example an existing Caddy installation can use:

```caddy
sip.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

## First administrator

Restrict initial web access to the trusted administrator until setup finishes.
Visit `/setup`; enter the administrator email and a 12–72 character password.
The wizard only creates the web administrator. It does not assign numbers or
create phone users. Twilio is supplied through the server environment.
There is no default administrator password. Setup closes after completion.
Use **Administrators** to create additional trusted web administrators.

## First phone and validation

Follow [Administration](ADMINISTRATION.md) to create a SIP user and explicitly
assign an existing number. Download the one-time phone setup file and configure
Linphone as in [Softphones](TWILIO_SOFTPHONES.md). Check health, use manual
inbound/outbound call and SMS tests, confirm two-way audio, inspect Activity
Log, and install/verify [backups](BACKUP.md) before relying on the system.
