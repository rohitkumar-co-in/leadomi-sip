# Leadomi SIP supported requirements

Internal trusted-admin console; custom SIP usernames/passwords; one phone and
one owned Twilio voice/SMS number per SIP identity; explicit connection review
before assigning/retrying; preserve working provider connections on ordinary
edits/startup; dedicated Twilio credential creation; TLS/SRTP calls; authenticated
SIP-to-SMS bridge; history with actor/from/to/time; disabling, usage controls,
health, audit and verified persistent backup/recovery.

Excluded product commitments: automatic number purchase, automatic takeover of
other inbound integrations, Messaging Service policy changes, high availability,
monetary spend caps, WebRTC/browser calling, conferencing/transfers, mobile MMS,
read receipts and typing indicators. Existing phone identities and data survive
rebranding. See [administrator guide](ADMINISTRATION.md) and
[operations](BUSINESS_OPERATIONS.md) for practical limits.
