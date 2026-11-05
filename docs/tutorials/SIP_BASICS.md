# SIP, audio and SMS in Leadomi SIP

SIP registers the phone and establishes calls. RTP carries voice; SRTP encrypts
it. TLS protects SIP signaling on TCP5061 by default, while UDP10000–10100
carries media by default. Both can be configured per deployment. HTTPS and the
SIP certificate each need a trusted hostname.

Twilio Voice connects telephone calls. Twilio SMS uses HTTP APIs/webhooks;
Asterisk's authenticated SIP MESSAGE bridge lets Linphone use the same assigned
number for text messages. SIP MESSAGE is not a carrier SMS transport by itself.

One local SIP identity maps to one owned Twilio number and one registered phone.
The phone password is separate from the Twilio SIP password and API Auth Token.
The stable digest realm is `gosip` for existing hash compatibility.

Use full `+country-code-number` for calls and `sip:+number@host` for messages.
Enable TLS/SRTP and disable CPIM in Linphone basic conversations. See
[phone setup](../TWILIO_SOFTPHONES.md). No browser WebRTC, conferences or transfer
UI is promised by this deployment. A registered phone is not proof that media
ports or carrier message delivery work; use manual two-way tests.
