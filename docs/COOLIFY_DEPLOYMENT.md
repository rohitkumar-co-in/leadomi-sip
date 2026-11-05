# Deploy Leadomi SIP with Coolify

## Existing production installation

Keep the existing Coolify application, service identifiers, persistent volumes,
phone passwords, dedicated SIP domain, and assigned numbers. The repository is
`https://github.com/rohitkumar-co-in/leadomi-sip`, branch `main`, Compose location
`/docker-compose.coolify.yml`. Branding does not require a new app or empty DB.

Production origin: `https://sip.leadomi.com`. Existing volume
`zgzcndvsggqprdlnm503jmpu_gosip-data` is supplied through `GOSIP_VOLUME_NAME`.
Do not rename/delete it. The two phone identities remain `gosipmobile` and
`gosipsecond`. Login, refresh, startup and deployment do not configure Twilio
numbers. Reopen phones after a restart; do not re-provision them to reconnect.

Before an update, create a host backup, record the current Git revision and
verify current health. Deploy the new main commit, wait for both services to
be healthy, check existing assignments/routes, trusted public TLS, SIP digest
authentication, and callback mappings read-only. Use manual calls/SMS for paid
acceptance tests. If reverting code, consider whether a schema migration is
backward compatible; never blindly restore a database into running containers.

## Fresh Coolify app

1. Configure DNS, required public ports, and root-owned SIP certificates as in
   [Installation](INSTALLATION.md). Prepare a dedicated Twilio SIP Domain and
   Credential List without touching working numbers elsewhere.
2. Create a new Docker Compose application from this repository/main and select
   `/docker-compose.coolify.yml`.
3. Create the intended **new** Go data volume (for example
   `docker volume create leadomi-sip-data`) and set `GOSIP_VOLUME_NAME` to it.
   The external volume must already exist. Never point a new app at an unrelated
   live database or delete a previous installation's volume.
4. Add all [configuration variables](CONFIGURATION.md) as runtime values.
   Use `GOSIP_PBX_URL=http://pbx:8088`. Assign the public HTTPS domain to the Go
   service's internal port8080. The PBX has direct TLS5061 and UDP media ports;
   it is not routed through the web reverse proxy.
5. Deploy, create the first administrator at `/setup`, and store the `SD…`/`CL…`
   metadata using the backend container terminal as described in Installation.
6. Create a SIP user, review the chosen number's existing connections, explicitly
   confirm assignment, download its phone file, and perform manual tests.
7. Install and verify the backup timer with your new PBX container prefix.

The Twilio Account SID/Auth Token belong in runtime environment, not Docker
build arguments or committed `.env` files. Keep the PBX unused API alias empty.
Do not expose private PBX API8088 or Go SIP5060 through Coolify port mappings.

## Certificate renewal and health

Coolify's HTTPS certificate and `/data/gosip-pbx/certs` must stay synchronized.
Use a root-only renewal export hook matching your proxy/certificate provider.
Never print private keys or application environment values in deployment logs.
Backend health: `/api/health`; PBX private health: `/health` on8088.
Authenticated **Dashboard** checks provider configuration, PBX health, and
latest backup. A health check is not proof of end-to-end carrier audio.
