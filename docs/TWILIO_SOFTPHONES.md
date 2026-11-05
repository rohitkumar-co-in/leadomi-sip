# Android softphone setup for Leadomi SIP

The supported tested path is Linphone on Android. Zoiper compatibility depends
on its edition's TLS/SRTP and SIP MESSAGE support; do not assume feature parity.
Twilio API credentials never belong on a phone.

## Linphone account fields

Create an ordinary SIP account using the downloaded setup file:

| Field | Value |
|---|---|
| SIP identity | `sip:YOUR_USERNAME@sip.example.com` |
| Authentication username | `YOUR_USERNAME` |
| Password | The custom or one-time generated **phone** password |
| Domain | `sip.example.com` |
| Registrar URI | `sip:sip.example.com:5061;transport=tls` |
| Outbound SIP Proxy URI | `sip:sip.example.com:5061;transport=tls` |
| Transport | TLS, port5061 |
| Media encryption | SRTP |
| Basic conversation CPIM | Off |

Set **both** proxy fields; using only a registrar can send calls/messages to the
wrong address. Keep the certificate hostname intact. Enable G.711 PCMU/PCMA
when client codec controls are available. Menu labels vary by Linphone version.
There is one registered phone per SIP user; registration on another phone can
replace the first phone's contact.

## Manual acceptance test with two phones

1. Keep Linphone open and registered on both phones, each with its own SIP user
   and assigned owned number. Turn off CPIM on both.
2. On phone A, dial phone B's assigned Twilio number in full `+…` format. Answer
   on B, speak on both sides, then hang up. Repeat B→A.
3. For SMS A→B, use `sip:+B_NUMBER@sip.example.com`. Confirm receipt in B's
   Linphone conversation and status in the administrator Activity Log.
4. Repeat SMS B→A. Confirm the sender is B's assigned number and actor B's SIP
   username. A SIP MESSAGE acknowledgment is not a carrier delivery/read receipt.
5. Test a normal external phone calling each Twilio number. Validate voicemail
   after an unanswered call if that route is configured.

These tests incur normal Twilio usage charges and are initiated manually by the
operator. No automated calls or SMS are required for a deployment smoke check.
Android battery restrictions can prevent background ringing; allow background
activity and reopen Linphone after a server restart.

Existing deployment: `sip.leadomi.com` uses usernames `gosipmobile` (number ending
6354) and `gosipsecond` (ending2200). Branding updates preserve those identifiers
and existing phone passwords. Do not assign their numbers again merely to test
the new UI.
