# Leadomi SIP supported business API

HTTPS origin `/api`. Web console calls use the HttpOnly session cookie; scripts
may use an authenticated bearer session. Business endpoints require an
administrator. Mutation origins must match the public/configured trusted origin.
Never log session tokens, passwords or provider responses containing secrets.

## Business endpoints

| Method / path | Purpose |
|---|---|
| GET `/business/accounts` | Local SIP users, assignments/state, registration, server/proxy |
| POST `/business/accounts` | Create and provision reviewed assignment |
| PUT `/business/accounts/{id}` | Edit/reset or explicitly reassign |
| POST `/business/accounts/{id}/disable` | Disable locally and revoke dedicated Twilio credential |
| GET `/business/numbers` | Read-only owned Twilio inventory and capabilities |
| GET `/business/number-review?number=%2B…` | Read-only webhook/application/service/route check |
| GET `/business/activity?kind=call&offset=0` | Call/SMS log; kind empty, call or sms;50records/page |
| GET `/business/audit` | Last100 provisioning/administrative workflow entries |
| GET `/business/status` | PBX health, provider configured, latest host backup |
| GET/PUT `/business/policy` | Allowed prefixes and SMS-per-minute limit |

Create request fields:

```json
{"name":"Agent name","username":"agent_custom","number":"+441234567890","password":"OPTIONAL_CUSTOM_PHONE_PASSWORD","number_review":"FINGERPRINT_FROM_REVIEW"}
```

Omit `password` or use empty for generation. Review response includes `number`,
`connections`, `fingerprint`, `blocked`. Explicitly confirm the described changes
with the operator before submitting that fingerprint. No provider/local user
writes occur if confirmation is missing/stale or the service check fails.
An overriding Messaging Service is blocked. The snapshot cannot detect all
outbound-only integrations and cannot lock Twilio against concurrent external
edits; avoid changing Twilio configuration during a confirmed assignment.

PUT fields: `name`, `number`, optional `reset_password`, optional `password`,
and `number_review` when assigning/retrying. A ready enabled account with the
same number preserves provider credentials/webhooks/routes. Username is fixed.
Successful create/reset returns the one-time phone password, server and proxy;
store that response privately. Provisioning operations have a120second HTTP
write deadline and90second server operation timeout. Partial failures return an
error state for retry; provider and database writes are not one transaction.

Common conflict codes: `NUMBER_ASSIGNED`, `USERNAME_EXISTS`,
`NUMBER_REVIEW_REQUIRED`, `NUMBER_CONNECTED`. Failed remote revocation returns
`REVOCATION_PENDING`; local disabling has already taken effect.

## Authentication and console data

`GET /setup/status`, `POST /setup/complete` provide fresh first-admin setup.
Setup closes after completion. `POST /auth/login`, `/auth/logout`, `GET /me`,
`PUT /me/password` manage persistent sessions. `/users` administrator CRUD is
separate from SIP identities. The supported UI uses read-only `/dids`, local
`/routes`, `/voicemails`, and system backup endpoints.

SMS: `GET /messages/conversations?did_id=ID`,
`GET /messages/conversation/{encoded-remote-number}?did_id=ID`, and
`POST /messages` with `did_id`, `to_number`, `body`, optional `media_urls`.
The UI selects an existing local DID. Administrator SMS actor is recorded.
Call/SMS Activity Log records include `kind`, `id`, `actor`, `direction`,
`from_number`, `to_number`, `status`, `occurred_at`, `duration`, `body`.
Historical actors are not guessed; incoming assigned accounts are labeled.

## Provider/private endpoints

Twilio POST callbacks: `/webhooks/voice/incoming`, `/voice/outgoing`,
`/voice/status`, `/voice/dial-complete`, `/sms/incoming`, `/sms/status`,
and `/voicemail/recording` under `/api/webhooks`. Signed callbacks are validated;
web sessions do not substitute for provider signatures. Outgoing domain voice
validates the dedicated Twilio account/domain/device and allowed destination.

Private PBX message submission uses the shared secret and the authenticated SIP
username. PBX credential/registration/backup APIs are private Docker traffic,
not public administration endpoints. Legacy device/provisioning/trunk/DID
mutation routes are blocked in business mode to preserve workflow consistency.
