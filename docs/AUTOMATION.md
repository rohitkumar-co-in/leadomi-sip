# n8n automation: missed-call text-back, AI replies and outreach

The SIP server keeps the Twilio number's webhooks so the phone keeps ringing
and receiving SMS. n8n is connected *through* the server, never directly to the
number.

## Configure

Set in Coolify and redeploy:

| Variable | Value |
|---|---|
| `GOSIP_AUTOMATION_KEY` | Random secret, at least 32 characters (`openssl rand -hex 24`) |
| `GOSIP_N8N_MISSED_CALL_URL` | n8n production webhook URL of the missed-call workflow |
| `GOSIP_N8N_SMS_URL` | n8n production webhook URL of the SMS assistant workflow |

In n8n create one **Header Auth** credential: name `X-Leadomi-Key`, value =
`GOSIP_AUTOMATION_KEY`. Use it on both workflows' Webhook triggers and on the
HTTP Request nodes that send SMS.

Then in the console **Settings → Missed-call text-back & AI replies**:

- **Demo mode** — on: calls are rejected as busy and the caller is texted at
  once; off: the phone rings 30 s and only unanswered calls are texted.
- **AI replies to SMS** — incoming SMS are forwarded to the assistant. When a
  person replies from the phone or Messages, the AI pauses for that contact for
  24 hours.

## Events the server sends to n8n

`POST` JSON with header `X-Leadomi-Key`, using Twilio's field names so
workflows read `{{ $json.body.From }}` and `{{ $json.body.Body }}`:

```json
{"Event":"missed_call","From":"+447700900123","To":"+447782226484","CallSid":"CA…","CallStatus":"no-answer","Demo":"false"}
{"Event":"sms","From":"+447700900123","To":"+447782226484","Body":"My boiler is broken","MessageSid":"SM…"}
```

Missed calls are reported once per call (busy, no answer, failed, cancelled or
demo mode). Withheld numbers and opted-out contacts are skipped.

## Sending SMS from n8n

```
POST https://sip.example.com/api/automation/sms
X-Leadomi-Key: <GOSIP_AUTOMATION_KEY>
Content-Type: application/json

{"to":"+447700900123","body":"Sorry we missed your call, how can we help?"}
```

The message is sent from the phone's number, appears in Messages, counts
toward the per-minute SMS limit and allowed-country policy, and is refused with
403 for contacts who texted STOP (START opts back in). Use the same endpoint for
outreach; send slowly and only to contacts who may lawfully be messaged.
