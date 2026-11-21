# New SIP server: complete setup checklist

Follow this top to bottom for a brand-new server. Every step exists because
skipping it caused a real problem before; the reason is noted in *italics*.
Replace `sip.example.com` with your hostname (e.g. `sip.leadomi.com`, or
`sid.leadomi.com` for a second server).

Time: about 45 minutes. When finished, the Dashboard **Setup check** is all green
except the optional items you skipped.

---

## 1. Server (AWS / VPS)

- [ ] Ubuntu VPS with Coolify installed (2 CPU, 2 GB RAM, 10 GB disk minimum).
- [ ] **Attach an Elastic IP** (EC2 → Elastic IPs → Allocate → Associate).
      *Without it the IP changes on every stop/start and calls connect with no sound.*
- [ ] Security group inbound rules:

| Type | Port | Source | Why |
|---|---|---|---|
| TCP | 80 | 0.0.0.0/0 | Certificate renewal (all HTTPS and SIP TLS) |
| TCP | 443 | 0.0.0.0/0 | Console, webhooks, n8n |
| TCP | 5061 | 0.0.0.0/0 | SIP for phones (first server) |
| UDP | 10000–10100 | 0.0.0.0/0 | Call audio (first server) |
| TCP | 5062 / UDP 11000–11100 | 0.0.0.0/0 | Only for a second server |
| TCP | 22, 8000, 6001, 6002 | your IP | SSH and Coolify |

- [ ] **Do not** open 5060. *Nothing uses it; it attracts SIP scanners.*

## 2. DNS

- [ ] `A` record `sip` → the Elastic IP.
- [ ] Cloudflare: **DNS only** (grey cloud). *The orange proxy does not carry SIP.*
- [ ] Second server on a non-5061 port only: `SRV` record `_sips._tcp.sid` →
      priority 0, weight 0, port 5062, target `sid.example.com`.

## 3. Twilio (in the account that owns the number)

- [ ] A number with **Voice and SMS**.
- [ ] **Voice → Manage → SIP domains → Create**, e.g. `leadomi.sip.twilio.com`:
  - [ ] **Secure media: ON** and **SIP registration: ON**.
        *Off → the phone user cannot be set up.*
  - [ ] Create a **Credential list** with one credential (username e.g.
        `leadomi`, a long alphanumeric password). Select it under **Voice
        authentication** *and* **SIP registration**.
  - [ ] **A call comes in:** `https://sip.example.com/api/webhooks/voice/outgoing`, **POST**.
        *Empty → outgoing calls fail.*
  - [ ] **Primary handler fails:** a TwiML Bin (Twilio requires it):
        `<Response><Say>Sorry, the line is unavailable. Please try again later.</Say></Response>`
  - [ ] Copy the domain SID `SD…` and credential list SID `CL…`.
- [ ] **Geo permissions** for every country you call or text
      (Voice → Settings → Geo permissions, Messaging → Settings → Geo permissions).
      *India unticked → calls/texts to +91 fail.*
- [ ] Optional **browser Dialer**:
  - [ ] Account → API keys & tokens → Create API key (Standard). Copy `SK…` and the secret (shown once).
  - [ ] Voice → Manage → TwiML Apps → Create: Voice Request URL
        `https://sip.example.com/api/webhooks/voice/browser`, POST. Leave status
        callback and the Messaging tab empty; caller name lookup off; do not
        allow dialing from other accounts. Copy `AP…`.

You do **not** set the number's webhooks yourself: the server sets them when the
phone user is created.

## 4. Coolify application

- [ ] New resource → this GitHub repository, branch `main`, build pack **Docker
      Compose**, compose file `/docker-compose.coolify.yml`.
- [ ] `gosip` service domain: `https://sip.example.com`.
- [ ] Environment variables (runtime; see [Configuration](CONFIGURATION.md)):

| Variable | Value |
|---|---|
| `TWILIO_ACCOUNT_SID` / `TWILIO_AUTH_TOKEN` | from the Twilio console home |
| `TWILIO_SIP_DOMAIN` | e.g. `leadomi.sip.twilio.com` |
| `TWILIO_SIP_DOMAIN_SID` / `TWILIO_CREDENTIAL_LIST_SID` | `SD…` / `CL…` |
| `PBX_TWILIO_USER` / `PBX_TWILIO_PASSWORD` | the credential from step 3 (also the Linphone username) |
| `GOSIP_PHONE_PASSWORD` | Linphone password, 12–72 chars, no spaces |
| `GOSIP_OUTBOUND_CALLER_ID` | the number, e.g. `+447700900000` |
| `GOSIP_PUBLIC_URL` / `GOSIP_CORS_ORIGINS` | `https://sip.example.com` |
| `GOSIP_PBX_SIP_DOMAIN` / `PBX_DOMAIN` | `sip.example.com` |
| `PBX_PUBLIC_IP` | **the Elastic IP** — must match DNS |
| `GOSIP_PBX_URL` | `http://pbx:8088` |
| `GOSIP_PBX_SECRET` | `openssl rand -hex 32` |
| `GOSIP_PBX_TRUNK_USER` / `GOSIP_PBX_TRUNK_PASSWORD` | e.g. `leadomi_incoming` / `openssl rand -hex 24` |
| `TZ` | e.g. `Asia/Kolkata` |
| Second server only: `GOSIP_PBX_SIP_PORT`, `PBX_RTP_PORT_START`, `PBX_RTP_PORT_END` | `5062`, `11000`, `11100` |
| Optional Dialer: `TWILIO_API_KEY_SID`, `TWILIO_API_KEY_SECRET`, `TWILIO_TWIML_APP_SID` | `SK…`, secret, `AP…` |
| Optional n8n: `GOSIP_AUTOMATION_KEY`, `GOSIP_N8N_MISSED_CALL_URL`, `GOSIP_N8N_SMS_URL` | see [Automation](AUTOMATION.md) |

- [ ] Do **not** define `GOSIP_PBX_UNUSED_API_CREDENTIAL`; leave `PBX_TWILIO_DEVICE_PASSWORDS` empty.
- [ ] **Deploy.** The phone server waits (up to 15 min) for the TLS certificate.

## 5. Console

- [ ] Open `https://sip.example.com/setup`, create the first administrator.
- [ ] **Dashboard → Setup check**: fix every red item (each one says how).
      *It catches a wrong IP, wrong webhooks, missing secure media, etc.*
- [ ] **Settings → Features status**: confirm what is active. Demo mode should
      be **Off** for real customers.

## 6. Linphone (each device)

Easiest: **Phone Setup → automatic setup link** → Linphone → Assistant →
*Fetch remote configuration* → paste → enter the password.

Manual, if needed:

- [ ] Username = `PBX_TWILIO_USER`, password = `GOSIP_PHONE_PASSWORD`, domain `sip.example.com`, transport **TLS**.
- [ ] **Proxy server URL** `<sip:sip.example.com:5061;transport=tls>` (both `< >`; use 5062 for a second server) and **Outgoing proxy ON**.
- [ ] Media encryption **SRTP**, mandatory **ON**. Codecs **PCMU and PCMA** on.
- [ ] Settings → Network: **Allow IPv6 OFF**. *On → no sound.*
- [ ] Settings → Chat: **CPIM OFF**. *On → texts fail (415).*
- [ ] Windows: allow the firewall prompt; microphone privacy allows desktop apps.
- [ ] Android: battery **Unrestricted**. iPhone: keep Linphone open.
- [ ] Dial numbers on the keypad as `+44…`; do not call saved SIP-address contacts.
- [ ] One device per account (a second device takes over the registration).

Ignore "Error retrieving devices". If the sign-in wizard errors and removes the
account, use the setup link instead.

## 7. Test (in this order)

- [ ] Linphone → your mobile: rings, both hear each other, hang up ends both sides.
- [ ] Your mobile → business number: Linphone rings; answer; hang up.
- [ ] Call and **decline**: the text-back arrives (if n8n is connected).
- [ ] Text in and out (Messages page and Linphone).
- [ ] Dialer page (if set up): call out; incoming pop-up answers.
- [ ] Simulator: a fake missed call and reply, no credits used.

If anything fails, see [Troubleshooting](TROUBLESHOOTING.md).

## 8. After setup

- [ ] Install the [backup timer](BACKUP.md).
- [ ] Note the Elastic IP; if it ever changes, update `PBX_PUBLIC_IP` on every SIP app and redeploy.
- [ ] Updating: deploy the new `main` commit, wait for healthy, run test 7 again.
