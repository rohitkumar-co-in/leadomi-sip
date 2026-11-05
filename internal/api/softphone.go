package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rohitkumar-co-in/leadomi-sip/internal/models"
)

var e164Destination = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

// sipEndpoint accepts only an ordinary user at the configured Twilio domain or
// one of its edge-specific hosts. Arbitrary SIP destinations are never dialed.
func sipEndpoint(value, domain string) (string, bool) {
	if domain == "" || !strings.HasPrefix(value, "sip:") {
		return "", false
	}
	parts := strings.Split(strings.SplitN(strings.TrimPrefix(value, "sip:"), ";", 2)[0], "@")
	if len(parts) != 2 {
		return "", false
	}
	host := strings.ToLower(parts[1])
	if hostname, port, found := strings.Cut(host, ":"); found {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", false
		}
		host = hostname
	}
	domain = strings.ToLower(domain)
	prefix := strings.TrimSuffix(domain, ".sip.twilio.com")
	if host != domain {
		if prefix == domain || !strings.HasPrefix(host, prefix+".sip.") || !strings.HasSuffix(host, ".twilio.com") {
			return "", false
		}
		edge := strings.TrimSuffix(strings.TrimPrefix(host, prefix+".sip."), ".twilio.com")
		if edge == "" || strings.ContainsAny(edge, ".:/") {
			return "", false
		}
	}
	username, err := url.PathUnescape(parts[0])
	return username, err == nil && username != "" && !strings.ContainsAny(username, "@;:/? \t\r\n")
}

// VoiceOutgoing bridges an authenticated Twilio SIP endpoint to the PSTN.
// Twilio authenticates SIP credentials; Leadomi SIP validates the signed webhook and
// permits only a known device, configured caller ID, and E.164 destination.
func (h *WebhookHandler) VoiceOutgoing(w http.ResponseWriter, r *http.Request) {
	if r.ParseForm() != nil || !h.validateSignature(r) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	sid, _ := h.deps.Config.TwilioCredentials()
	if r.FormValue("AccountSid") != sid {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	username, valid := sipEndpoint(r.FormValue("From"), h.deps.Config.TwilioSIPDomain)
	device, err := h.deps.DB.Devices.GetByUsername(r.Context(), username)
	if !valid || err != nil {
		h.respondTwiML(w, h.errorTwiML("Unknown softphone"))
		return
	}
	number, valid := sipEndpoint(r.FormValue("To"), h.deps.Config.TwilioSIPDomain)
	if !valid || !e164Destination.MatchString(number) {
		h.respondTwiML(w, h.errorTwiML("Dial the full international number starting with plus"))
		return
	}
	callerID, err := deviceOutboundNumber(r.Context(), h.deps, username)
	if err != nil {
		h.respondTwiML(w, h.errorTwiML("Outbound caller ID is not configured"))
		return
	}
	if !businessDestinationAllowed(r.Context(), h.deps, number) {
		h.respondTwiML(w, h.errorTwiML("This destination is blocked by your business usage policy"))
		return
	}
	did, err := h.deps.DB.DIDs.GetByNumber(r.Context(), callerID)
	if err != nil || !did.VoiceEnabled || !e164Destination.MatchString(did.Number) {
		h.respondTwiML(w, h.errorTwiML("Outbound caller ID is not configured"))
		return
	}
	h.recordVoiceCall(r.Context(), r.FormValue("CallSid"), "outbound", did.Number, number, did.ID, &device.ID)
	h.respondTwiML(w, `<Response><Dial callerId="`+escapeXML(did.Number)+`"><Number>`+escapeXML(number)+`</Number></Dial></Response>`)
}

func (h *WebhookHandler) recordVoiceCall(ctx context.Context, callSID, direction, from, to string, didID int64, deviceID *int64) {
	if callSID == "" {
		return
	}
	if _, err := h.deps.DB.CDRs.GetByCallSID(ctx, callSID); err == nil {
		return
	}
	if err := h.deps.DB.CDRs.Create(ctx, &models.CDR{
		CallSID: callSID, Direction: direction, FromNumber: from, ToNumber: to,
		DIDID: &didID, DeviceID: deviceID, StartedAt: time.Now(), Disposition: "ringing",
	}); err != nil {
		slog.Error("Unable to record voice call", "error", err)
	}
}

// VoiceDialComplete falls back to voicemail only when the softphone did not answer.
func (h *WebhookHandler) VoiceDialComplete(w http.ResponseWriter, r *http.Request) {
	if r.ParseForm() != nil || !h.validateSignature(r) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	switch r.FormValue("DialCallStatus") {
	case "busy", "no-answer", "failed":
		id, err := strconv.ParseInt(r.URL.Query().Get("DidId"), 10, 64)
		if err == nil {
			if _, err := h.deps.DB.DIDs.GetByID(r.Context(), id); err == nil {
				h.respondTwiML(w, h.voicemailTwiML(r.Context(), id, r.FormValue("From")))
				return
			}
		}
	}
	h.respondTwiML(w, `<Response><Hangup/></Response>`)
}
