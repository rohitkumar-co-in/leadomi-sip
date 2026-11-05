# Add a second Leadomi SIP app in Coolify

This guide runs another independent Leadomi SIP installation from the private
GitHub repository on the same VPS. It leaves the existing `GoSIP` Coolify app,
its environment, database, PBX data, certificates, Twilio settings, and ports
alone.

> **Important:** Create a new Coolify application resource. Do not edit or
> redeploy the existing app to change it into the second installation. Do not
> reuse its `GOSIP_VOLUME_NAME`, SIP/RTP ports, or certificate directory.

## Values for the second installation

Replace the example hostnames and placeholders with your own values. Keep
secrets in Coolify's Environment Variables UI; do not commit them to Git.

| Variable | Second app value | Notes |
|---|---|---|
| `TWILIO_ACCOUNT_SID` | Second installation's Twilio Account SID | Use the account that owns the second app's phone numbers. |
| `TWILIO_AUTH_TOKEN` | That account's Auth Token | Secret; Go backend only. |
| `TWILIO_SIP_DOMAIN` | Its Twilio SIP Domain, e.g. `company2.sip.twilio.com` | Create/configure this in the second Twilio account. |
| `GOSIP_PUBLIC_URL` | `https://sip2.example.com` | HTTPS URL for this app. |
| `GOSIP_CORS_ORIGINS` | `https://sip2.example.com` | Match the app's browser origin. |
| `GOSIP_PBX_URL` | `http://pbx:8088` | Keep; this is the private Compose service URL. |
| `GOSIP_PBX_SECRET` | A newly generated random secret | Do not reuse the first app's secret. |
| `GOSIP_PBX_SIP_DOMAIN` | `sip2.example.com` | Hostname only; the public port is configured separately. |
| `GOSIP_PBX_SIP_PORT` | `5062` | Public SIP over TLS port for this instance. |
| `GOSIP_PBX_TRUNK_USER` | A new private trunk username | Keep distinct from the first app. |
| `GOSIP_PBX_TRUNK_PASSWORD` | A newly generated password | Do not reuse the first app's password. |
| `PBX_DOMAIN` | `sip2.example.com` | Same hostname as `GOSIP_PBX_SIP_DOMAIN`. |
| `PBX_PUBLIC_IP` | The VPS public IPv4 address | The same server IP can host both instances. |
| `PBX_RTP_PORT_START` | `11000` | Second instance media range. |
| `PBX_RTP_PORT_END` | `11100` | Second instance media range. |
| `PBX_CERTS_HOST_PATH` | `/data/gosip-pbx-second/certs` | Separate host directory with this SIP hostname's certificate. |
| `PBX_TWILIO_USER` | `leadomi_second_bootstrap` | Create this username in the second Twilio Credential List. |
| `PBX_TWILIO_PASSWORD` | A new password for that Twilio credential | It must match the credential in the second Twilio account. |
| `PBX_TWILIO_DEVICE_PASSWORDS` | Leave empty | Only populate this when deliberately restoring existing device credentials. |
| `GOSIP_OUTBOUND_CALLER_ID` | A number owned by the second Twilio account | Use E.164 format, e.g. `+441234567890`. |
| `GOSIP_VOLUME_NAME` | `leadomi-sip-second-data` | New external Docker volume; create it before deployment. |
| `TZ` | `Asia/Kolkata` | Set the desired timezone. |

Generate new random values for all passwords and secrets. The bootstrap SIP
username can technically be reused across separate Twilio accounts, but a
distinct name makes it easier to identify which installation it belongs to.

### Leave these Compose values alone

Do not add `GOSIP_DATA_DIR`, `GOSIP_DB_PATH`, `GOSIP_HTTP_PORT`,
`GOSIP_SIP_PORT`, `GOSIP_LOG_LEVEL`, or `GOSIP_DEV_MODE` as overrides for this
deployment. The Compose file sets the app's internal data directory and HTTP
port, and the standalone Go SIP listener is not the supported phone service.
The phone-facing port is `GOSIP_PBX_SIP_PORT=5062` above.

## Create the second Coolify application

1. In Coolify, create a **new application resource** (optionally in a separate
   project) on the same VPS. Connect the private GitHub repository using the
   GitHub source/integration already authorized for Coolify. Select branch
   `main` and Compose file `/docker-compose.coolify.yml`.
2. Confirm the application name and generated resource/stack are new. Do not
   open the existing `GoSIP` app to edit its variables or Compose settings.
3. Create the new external data volume on the VPS before deploying:

   ```sh
   docker volume inspect leadomi-sip-second-data >/dev/null 2>&1 && {
     echo 'That volume already exists. Choose a fresh name; do not reuse it.'
     exit 1
   }
   docker volume create leadomi-sip-second-data
   ```

   Set `GOSIP_VOLUME_NAME=leadomi-sip-second-data`. Never set it to the volume
   used by the existing app. The PBX state volume is Compose-project scoped, so
   keeping this as a distinct Coolify application also keeps PBX state separate.
4. Point DNS for the second HTTPS and SIP hostname (for example
   `sip2.example.com`) to the VPS. Configure that hostname on the Go service at
   internal port `8080`.
5. Create `/data/gosip-pbx-second/certs` on the VPS and install a trusted
   certificate for `sip2.example.com` as `fullchain.pem` and `privkey.pem`.
   Keep the directory root-owned, set the private key to mode `0600`, and
   arrange renewal for this separate certificate directory.
6. Enter the second app's environment values from the table. Verify the new
   Twilio account's SIP Domain and Credential List are configured, and that
   the bootstrap username/password match that list. Do not copy secret values
   from the first app.
7. In the second Twilio account, enable secure calling and SIP registration
   for its dedicated SIP Domain. Attach its Credential List to both **Calls**
   and **Registrations**, add the bootstrap credential from the table, and set
   the domain's outgoing Voice URL to
   `https://sip2.example.com/api/webhooks/voice/outgoing` with method `POST`.
   Record that Domain SID and Credential List SID for the second app's setup.
   Leave the first account's domain and phone-number callbacks unchanged.
8. In the VPS firewall and AWS security group, allow **TCP 5062** and **UDP
   11000–11100**. Keep the existing app's **TCP 5061** and **UDP
   10000–10100** rules unchanged. HTTPS for both web apps can use TCP 443 through
   Coolify's proxy. Compose maps public TCP 5062 to Asterisk's internal TLS
   listener on 5061; the configured RTP range is used both inside and outside
   the container.
9. Deploy the new app. Complete its first-admin and Twilio metadata setup, then
   create SIP users using phone numbers owned by the second Twilio account.
   Do not assign a number already routed through the first app.

## Verify before giving phones to users

- In Coolify, confirm the **new app** is `running:healthy` and its deployment
  used the expected `main` commit.
- Open `https://sip2.example.com/api/health` and confirm it reports healthy.
- Confirm the first app still uses TCP 5061, UDP 10000–10100, its original
  volume, and its original certificate directory.
- Download a setup file from the second app. It should show the second SIP
  hostname and TLS port `5062`.
- Confirm the AWS/VPS firewall allows TCP 5062 and UDP 11000–11100 before
  testing registration and two-way audio.

If a setting in Coolify would require changing the existing `GoSIP` resource,
stop and return to the new application resource instead. Coolify applies
container-affecting environment changes on deploy/restart; keep the first app's
environment unchanged. See [Coolify environment variable behavior](https://coolify.io/docs/applications/configuration/environment-variables).
