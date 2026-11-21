package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/rohitkumar-co-in/leadomi-sip/internal/config"
)

// The browser Dialer uses the Twilio Voice JavaScript SDK. The console hands
// a short-lived access token to a signed-in administrator; Twilio then calls
// /api/webhooks/voice/browser (the TwiML App's Voice URL) for outgoing calls,
// and incoming calls ring the client identity alongside the phone.

const dialerIdentity = "leadomi-dialer"

func dialerConfigured(cfg *config.Config) bool {
	return resourceSID.MatchString(cfg.TwilioTwiMLAppSID) && cfg.TwilioAPIKeySID != "" && cfg.TwilioAPIKeySecret != ""
}

// dialerAccessToken builds a Twilio Access Token (JWT, HS256) with a Voice grant.
func dialerAccessToken(cfg *config.Config, now time.Time, ttl time.Duration) (string, error) {
	account, _ := cfg.TwilioCredentials()
	enc := func(v interface{}) (string, error) {
		b, err := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b), err
	}
	header, err := enc(map[string]string{"typ": "JWT", "alg": "HS256", "cty": "twilio-fpa;v=1"})
	if err != nil {
		return "", err
	}
	payload, err := enc(map[string]interface{}{
		"jti": cfg.TwilioAPIKeySID + "-" + strconv.FormatInt(now.Unix(), 10),
		"iss": cfg.TwilioAPIKeySID,
		"sub": account,
		"nbf": now.Unix(),
		"exp": now.Add(ttl).Unix(),
		"grants": map[string]interface{}{
			"identity": dialerIdentity,
			"voice": map[string]interface{}{
				"incoming": map[string]bool{"allow": true},
				"outgoing": map[string]string{"application_sid": cfg.TwilioTwiMLAppSID},
			},
		},
	})
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, []byte(cfg.TwilioAPIKeySecret))
	mac.Write([]byte(header + "." + payload))
	return header + "." + payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// DialerToken serves GET /api/business/dialer/token.
func (h *BusinessHandler) DialerToken(w http.ResponseWriter, r *http.Request) {
	cfg := h.deps.Config
	if !dialerConfigured(cfg) {
		WriteJSON(w, http.StatusOK, map[string]interface{}{"configured": false})
		return
	}
	token, err := dialerAccessToken(cfg, time.Now(), time.Hour)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "dialer_token", "Could not create a dialer token", nil)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	WriteJSON(w, http.StatusOK, map[string]interface{}{"configured": true, "token": token, "ttl": 3600, "number": phoneNumberFor(r.Context(), h.deps)})
}

// VoiceBrowser answers outgoing calls from the browser Dialer (TwiML App Voice URL).
func (h *WebhookHandler) VoiceBrowser(w http.ResponseWriter, r *http.Request) {
	if r.ParseForm() != nil || !h.validateSignature(r) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	sid, _ := h.deps.Config.TwilioCredentials()
	if r.FormValue("AccountSid") != sid || r.FormValue("From") != "client:"+dialerIdentity || !dialerConfigured(h.deps.Config) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	number := r.FormValue("To")
	if !e164Destination.MatchString(number) {
		h.respondTwiML(w, h.errorTwiML("Dial the full international number starting with plus"))
		return
	}
	if !businessDestinationAllowed(r.Context(), h.deps, number) {
		h.respondTwiML(w, h.errorTwiML("This destination is blocked by your business usage policy"))
		return
	}
	did, err := h.deps.DB.DIDs.GetByNumber(r.Context(), phoneNumberFor(r.Context(), h.deps))
	if err != nil || !did.VoiceEnabled || !e164Destination.MatchString(did.Number) {
		h.respondTwiML(w, h.errorTwiML("Outbound caller ID is not configured"))
		return
	}
	h.recordVoiceCall(r.Context(), r.FormValue("CallSid"), "outbound", did.Number, number, did.ID, nil)
	h.respondTwiML(w, `<Response><Dial answerOnBridge="true" callerId="`+escapeXML(did.Number)+`" action="/api/webhooks/voice/outgoing-complete"><Number>`+escapeXML(number)+`</Number></Dial></Response>`)
}
