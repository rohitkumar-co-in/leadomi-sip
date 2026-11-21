# Changelog

## 2026-11 — single-server rebuild, automation and second server

### Browser Dialer
- New **Dialer** page: keypad, call, hang up, mute, timer, from any browser.
  Incoming calls ring the browser and Linphone together, with an Answer/Decline
  pop-up on every console page. Uses the Twilio Voice JS SDK (`TWILIO_API_KEY_*`,
  `TWILIO_TWIML_APP_SID`); Setup check verifies the TwiML App.

### Deployment
- Fresh one-app Coolify install. The PBX takes its TLS certificate from Coolify's
  Traefik store automatically and waits for it before starting. Data volumes are
  created automatically.
- The phone user is defined by environment (`PBX_TWILIO_USER`,
  `GOSIP_PHONE_PASSWORD`) and created on start; SIP Users, Routes and Excluded
  Numbers pages were removed.
- Ports are configurable per server (`GOSIP_PBX_SIP_PORT`, `PBX_RTP_PORT_*`), so a
  second independent server can share the VPS (e.g. 5062 and 11000–11100).
- Linphone remote-provisioning link on Phone Setup for servers on a non-5061 port
  (no password inside, unguessable URL).

### Calls
- Outgoing, forwarded and ring-phone calls use `answerOnBridge`: callers are
  only connected and billed when the phone answers, and cancelling stops ringing
  at once.
- Phone-originated calls record how they end; calls stuck non-final for 4 h can
  be deleted.

### Messages
- Fixed a 500 on the second outgoing text (empty Twilio message IDs).
- Messages page needs no number picker when there is one number.

### Automation (n8n, Mike Garages demo)
- Missed-call text-back and AI booking via n8n, through the SIP server
  (`/api/automation/sms`, header `X-Leadomi-Key`).
- The AI answers **only** people who called (24 h session after a missed call);
  outreach replies go to a human. A human reply pauses the AI for 24 h; 12-turn
  cap; STOP/START; n8n ends the session after a booking.
- Demo mode: reject every call and send the text-back instantly.
- Simulator page: fake calls and texts from `+447700900xxx`, never sent to Twilio.

### Console
- New Dashboard tiles, Demo-mode banner and a read-only **Setup check**
  (Twilio account, SIP domain, security, webhooks, credential list, number,
  PBX, phone user, registration, and **call audio address** vs DNS).
- Phone Setup page with Windows/Android/iPhone guides and copy buttons.
- Readable Activity log, "Use for phone" number selection, clickable logo.

### Documentation
- [Troubleshooting](docs/TROUBLESHOOTING.md): every problem met during setup
  and its fix.
- Coolify guide: Elastic IP warning, firewall list, second server on the same
  VPS, troubleshooting table.
