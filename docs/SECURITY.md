# Leadomi SIP security notes

All application responses, including API/auth failures and static assets, carry
`X-Robots-Tag: noindex, nofollow`. The console HTML has matching robots metadata.
`robots.txt` allows crawling so search engines can read these exclusion rules;
blocking crawls would prevent discovery of `noindex`. These are search-engine
directives; administrator authentication remains the access boundary. Existing
search listings require recrawling before removal.

The supported business console is administrator-only. Password changes revoke
persisted/cached sessions. Mutating console origins are restricted. Twilio
callbacks are signed; the private PBX bridge authenticates sender identities.
Phone signaling uses TLS5061 and SRTP media. API credentials are backend runtime
environment values, excluded from the SQLite credential rows and PBX container.
Phone passwords are digest hashes; generated provider SIP credentials persist
in a root-only PBX file. Local secrets and backups are never public web assets.

Number inventory/review is read-only. Explicit reviewed assignment is required
before writes. Active local ownership is enforced; external inbound connections
are displayed; Messaging Service inbound overrides block automatic changes.
Ready-user name/password edits preserve provider credentials and routes.
Attribution snapshots preserve new call/SMS actor labels after reassignment.

The previous release's Go vulnerability scan reported zero known reachable
vulnerabilities, with remaining unused dependency advisories. Re-run scans when
updating dependencies; that result does not establish whole-system security.
Production verification uses read-only APIs, isolated tests and non-calling SIP
authentication checks, with manual paid end-to-end acceptance tests by operator.

Limits: one VPS, local-only backups until an external destination is configured,
trusted administrator access to all history, root access to deployment secrets,
no monetary spending cap, external provider writes not transactional with DB,
no complete detection of outbound-only number integrations, Android background
reachability limitations. Keep OS/images patched and review Twilio usage.

See [operations](BUSINESS_OPERATIONS.md) and [backup](BACKUP.md).
