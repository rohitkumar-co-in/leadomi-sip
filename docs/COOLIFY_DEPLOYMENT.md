# Deploy Leadomi SIP with Coolify

Leadomi SIP runs as **one** Coolify application per VPS. Repository
`https://github.com/rohitkumar-co-in/leadomi-sip`, branch `main`, build pack
Docker Compose, Compose location `/docker-compose.coolify.yml`.

## Fresh install

### 1. Before you start

- Delete any previous Leadomi SIP / GoSIP app on this VPS first. Its PBX holds
  TCP 5061 and UDP 10000–10100, so a new app cannot start until it is gone.
- DNS: an `A` record for your hostname (e.g. `sip.example.com`) pointing at the
  VPS public IP.
- Firewall / cloud security group: allow TCP 80, TCP 443, **TCP 5061** and
  **UDP 10000–10100**.
- Twilio: create the dedicated SIP Domain, Credential List and bootstrap
  credential described in [Installation](INSTALLATION.md#dedicated-twilio-resources).
  Note the Domain SID (`SD…`) and Credential List SID (`CL…`).

### 2. Create the application

1. Coolify → **+ New resource → Private repository (GitHub App)** or **Public
   repository**, choose this repository, branch `main`.
2. Build pack **Docker Compose**, Compose location `/docker-compose.coolify.yml`.
3. In the `gosip` service's domain field enter `https://sip.example.com`.
   Use the **same** hostname for `PBX_DOMAIN` and `GOSIP_PBX_SIP_DOMAIN`: the PBX
   reuses the certificate that Coolify's proxy obtains for this domain.

### 3. Environment variables

Add these as runtime variables (see [Configuration](CONFIGURATION.md)):

| Variable | Value |
|---|---|
| `TWILIO_ACCOUNT_SID` | `AC…` |
| `TWILIO_AUTH_TOKEN` | Twilio auth token |
| `TWILIO_SIP_DOMAIN` | e.g. `leadomi.sip.twilio.com` |
| `TWILIO_SIP_DOMAIN_SID` | `SD…` |
| `TWILIO_CREDENTIAL_LIST_SID` | `CL…` |
| `GOSIP_PUBLIC_URL` | `https://sip.example.com` |
| `GOSIP_CORS_ORIGINS` | `https://sip.example.com` |
| `GOSIP_PBX_URL` | `http://pbx:8088` |
| `GOSIP_PBX_SECRET` | `openssl rand -hex 32` |
| `GOSIP_PBX_SIP_DOMAIN` | `sip.example.com` |
| `PBX_DOMAIN` | `sip.example.com` |
| `PBX_PUBLIC_IP` | VPS public IPv4 |
| `GOSIP_PBX_TRUNK_USER` | e.g. `leadomi_incoming` |
| `GOSIP_PBX_TRUNK_PASSWORD` | `openssl rand -hex 24` |
| `PBX_TWILIO_USER` | bootstrap credential username from Twilio; also the phone's SIP username |
| `PBX_TWILIO_PASSWORD` | bootstrap credential password from Twilio |
| `GOSIP_OUTBOUND_CALLER_ID` | the phone's owned Twilio number in E.164, e.g. `+447700900000` |
| `GOSIP_PHONE_PASSWORD` | the phone (Linphone) password, 12–72 characters, no spaces |
| `GOSIP_PHONE_NAME` | optional display name for the phone user |
| `TZ` | e.g. `Asia/Kolkata` |

Do **not** define `GOSIP_PBX_UNUSED_API_CREDENTIAL`. Leave
`PBX_TWILIO_DEVICE_PASSWORDS` empty. Ports default to 5061 and 10000–10100.

### 4. Deploy and finish setup

1. **Deploy.** Data volumes are created automatically. The PBX waits (up to 15
   minutes) until Coolify's proxy has a certificate for `PBX_DOMAIN`, then starts.
2. Open `https://sip.example.com/setup` and create the first administrator.
3. The server creates the phone user by itself from `PBX_TWILIO_USER`,
   `GOSIP_OUTBOUND_CALLER_ID` and `GOSIP_PHONE_PASSWORD`, and points that
   number's call/SMS webhooks at this server. The **Dashboard** shows its status
   and the Linphone settings. There is no SIP Users page: to change the number or
   password, edit the variables and redeploy. A restart of an already set-up
   phone user does not change anything in Twilio.
4. Set the Twilio SIP Domain's Voice URL to
   `https://sip.example.com/api/webhooks/voice/outgoing` (POST) if not already.
5. Configure Linphone, then test an inbound and outbound call and SMS.
6. Install the [backup timer](BACKUP.md).

## Updating

Record the current Git revision and take a backup. Deploy the new `main`
commit, wait for both services to be healthy, and run a manual call/SMS test.
Never delete or empty the data volumes of a working installation, and never
restore a database into running containers.

The Twilio Account SID/Auth Token belong in runtime environment, not Docker
build arguments or committed `.env` files. Do not expose private PBX API 8088
or Go SIP 5060 through Coolify port mappings.

## Certificate renewal and health

The PBX reads Coolify's Traefik ACME store read-only
(`/data/coolify/proxy/acme.json`, override the directory with
`PBX_PROXY_HOST_DIR`) and copies the certificate for `PBX_DOMAIN` into its own
volume, checking every 10 seconds, so proxy renewals reach the SIP TLS port
automatically. If no proxy certificate exists, it falls back to
`fullchain.pem`/`privkey.pem` in `PBX_CERTS_HOST_PATH`.

Never print private keys or application environment values in deployment logs.
Backend health: `/api/health`; PBX private health: `/health` on 8088.
Authenticated **Dashboard** checks provider configuration, PBX health, and
latest backup. A health check is not proof of end-to-end carrier audio.
