# Troubleshooting

Setting up from scratch? Use the [setup checklist](SETUP_CHECKLIST.md) first.

These are real problems seen while setting up Leadomi SIP, with the fix for each.
Run the **Setup check** on the Dashboard first: it reads Twilio and the server
(read-only) and names most of these problems directly.

## Server and network

| Problem | Cause | Fix |
|---|---|---|
| Calls ring and connect but **no sound at all**, both ways | The VPS public IP changed (AWS instance stopped/started without an Elastic IP). `PBX_PUBLIC_IP` still holds the old IP, so audio goes to the wrong address. Setup check shows **Call audio address** in red. | Set `PBX_PUBLIC_IP` to the current IP on **every** SIP app in Coolify and redeploy. Attach an **AWS Elastic IP** so it stops changing, then update DNS. |
| PBX never starts after a fresh deploy | It waits (up to 15 min) for Coolify's proxy to get a TLS certificate for `PBX_DOMAIN`. | Check the DNS A record points at the VPS, Cloudflare is **DNS only**, and TCP 80/443 are open. |
| Which ports to open | | TCP 80 (certificate renewal), 443, 5061 (+5062 for a second server), UDP 10000–10100 (+11000–11100). **Do not** open 5060: nothing uses it and it attracts SIP scanners. Restrict 22, 8000, 6001, 6002 to your own IP if you can. |
| Log full of `INVITE ... No matching endpoint found` from unknown IPs | Internet SIP scanners. Harmless: they are rejected. | Optional: block the IP in the security group. |

## Twilio configuration

| Problem | Cause | Fix |
|---|---|---|
| Phone user setup fails | **Secure media** is off on the Twilio SIP domain. | Enable Secure media and SIP registration on the SIP domain. |
| Outgoing calls fail immediately | SIP domain "A call comes in" URL is empty. | Set it to `https://<host>/api/webhooks/voice/outgoing` (POST). Twilio also requires a **Primary handler fails** URL: use a TwiML Bin, e.g. `<Response><Say>Sorry, the line is unavailable.</Say></Response>`. |
| Calls to India (or another country) fail; texts to India not delivered | Twilio geo permissions. | Twilio → Voice → Settings → Geo permissions, and Messaging → Settings → Geo permissions: tick the country. Do it in **each** Twilio account. |
| Testing replies costs credits / phone cannot send international SMS | | Use the **Simulator** page: fictional UK numbers `+447700900000`–`+447700900999` are never sent to Twilio. |

## Linphone

| Problem | Cause | Fix |
|---|---|---|
| Registration fails on a server whose port is not 5061 (e.g. second server on 5062) | Linphone's sign-in form has no port field. Typing `host:5062` as the domain produces an invalid address and the server answers 401. | Use the **automatic setup link** on Phone Setup (Assistant → Fetch remote configuration), or set **Proxy server URL** to `<sip:host:5062;transport=tls>` with **Outgoing proxy** ON, or add DNS SRV `_sips._tcp.host` → 0 0 5062 host. |
| Proxy set but still not registering | The proxy URL is missing the closing `>`. | Enter it exactly, with both `<` and `>`. |
| Sign-in screen shows an error and the account disappears | The Linphone login wizard can sign in and then remove the account. | Configure the account via the setup link or by editing an existing account; do not rely on the wizard. |
| "Error retrieving devices" | Linphone's device list only works for linphone.org accounts. | Ignore it. |
| No sound or one-way sound with Windows/mobile data | **Allow IPv6** is on; the server is IPv4 only. | Settings → Network → Allow IPv6 OFF, restart Linphone. |
| You hear the other side but they do not hear you | Microphone. | Linphone → Settings → Audio: pick the right microphone; check mute; Windows → Privacy → Microphone: allow desktop apps. |
| "User cannot accept your call" | Media encryption is not SRTP, or PCMU/PCMA disabled. | Media encryption = SRTP (mandatory); enable PCMU and PCMA. |
| Call drops after ~30 s / no ACK | Outbound proxy off. | Turn **Outgoing proxy** ON. |
| A call "keeps loading" and never reaches the server | Linphone dialled a saved contact or history entry that holds a SIP address (e.g. `sid@sid.example.com`) instead of the number. | Type the number on the dial pad in `+44…` format, or fix the contact's address. |
| Sending a text fails (server log: `415 Unsupported Media Type`) | CPIM is on. | Settings → Chat → "Use CPIM in basic conversations" OFF. |
| Second Linphone signs in and the first stops ringing | One registered device per phone user. | Use one device per server account. |

## Browser Dialer

| Problem | Cause | Fix |
|---|---|---|
| Dialer says "not set up yet" | API key / TwiML App variables missing. | Follow *Browser Dialer* in the Coolify guide and redeploy. |
| Dialer will not start / no sound | Microphone blocked. | Click the lock icon in the address bar → allow Microphone, reload. |
| Browser calls fail at once | TwiML App Voice URL wrong. | Setup check → *Browser dialer*; set it to `/api/webhooks/voice/browser` (POST). |
| Incoming call does not ring the browser | The console tab is closed or the phone is locked. | Keep a console tab open; Linphone still rings. |

## Settings and features

| Problem | Cause | Fix |
|---|---|---|
| Not sure whether n8n, AI, Demo mode, Simulator or the Dialer is active | | Settings → **Features status** shows each as active, off or not set up, and which variables are missing. |
| Simulator page missing from the menu | Simulator switched off. | Settings → Simulator ON. |
| Second server shows n8n "Not set up" | Expected when that server has no automation. | Nothing to fix. |

## Calls

| Problem | Cause | Fix |
|---|---|---|
| Every incoming call is rejected ("address incomplete" / busy) and the caller gets the text-back | **Demo mode** is on. | Settings → Demo mode OFF. Missed (unanswered or declined) calls still get the text-back. |
| Caller sees "connected" and the timer runs while the phone is still ringing | Older build: the ring-phone `<Dial>` lacked `answerOnBridge`. | Fixed; redeploy the latest `main`. |
| Cancelling an outgoing call keeps the other phone ringing | Older build: outgoing `<Dial>` lacked `answerOnBridge`. | Fixed; redeploy. |
| Hang-up reaches the other side 1–3 s later | Normal when the call passes through two Twilio accounts and the phone network. | Nothing to fix. |
| Call history entries stuck on "ringing" cannot be deleted | Older build had no Dial completion action. | Fixed; calls still non-final after 4 hours can be deleted. |
| Second outgoing text returns 500 | Older build stored empty Twilio message IDs as duplicates. | Fixed by migration 017. |

## Automation (n8n)

| Problem | Cause | Fix |
|---|---|---|
| AI replies to people who only answered an outreach text | | By design the AI replies only within 24 h of a **missed call**; other replies go to a human in Messages. Check `GOSIP_N8N_*` are set and the workflows use the "Leadomi SIP key" header credential. |
| AI keeps chatting after a booking | | n8n calls `POST /api/automation/session/end` after booking; there is also a 12-turn cap and STOP/START handling. A human reply pauses the AI for 24 h. |
| Google Sheet columns land in the wrong place | Sheet headers have trailing spaces (`Phone `, `Name `…) or field names differ. | Map the n8n fields (Registration, Make/Model, Job, Drop-off Date, Drop-off Time) to the exact header text. |
