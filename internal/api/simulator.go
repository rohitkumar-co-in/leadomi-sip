package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rohitkumar-co-in/leadomi-sip/internal/models"
)

// The simulator lets an administrator act as a customer from a fictional
// number, exercising the real call/SMS/automation paths without a phone.
//
// Only Ofcom's reserved drama range +447700900000-+447700900999 is accepted:
// those numbers can never belong to anyone. Messages sent to them are never
// passed to Twilio; they are stored with status "simulated".

var simulatorNumberPattern = regexp.MustCompile(`^\+447700900[0-9]{3}$`)

func isSimulatorNumber(number string) bool { return simulatorNumberPattern.MatchString(number) }

func simulatorSID(prefix string) string {
	var b [16]byte
	rand.Read(b[:])
	return prefix + hex.EncodeToString(b[:])
}

// SimulatorHandler serves POST /api/business/simulator.
type SimulatorHandler struct{ deps *Dependencies }

func NewSimulatorHandler(deps *Dependencies) *SimulatorHandler { return &SimulatorHandler{deps: deps} }

// Simulate accepts {"action":"missed_call"|"sms","from":"+447700900123","body":"..."}.
func (h *SimulatorHandler) Simulate(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Action string `json:"action"`
		From   string `json:"from"`
		Body   string `json:"body"`
	}
	if automationSetting(r.Context(), h.deps, simulatorOffKey) {
		WriteError(w, http.StatusForbidden, "simulator_disabled", "The Simulator is turned off in Settings", nil)
		return
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil || !isSimulatorNumber(input.From) {
		WriteValidationError(w, "Use a simulator number from +447700900000 to +447700900999", nil)
		return
	}
	ctx := r.Context()
	did, err := h.deps.DB.DIDs.GetByNumber(ctx, phoneNumberFor(ctx, h.deps))
	if err != nil {
		WriteError(w, http.StatusServiceUnavailable, ErrCodeBadRequest, "The phone's number is not set up yet", nil)
		return
	}
	switch input.Action {
	case "missed_call":
		callSID := simulatorSID("CASIM")
		webhooks := NewWebhookHandler(h.deps)
		webhooks.recordVoiceCall(ctx, callSID, "inbound", input.From, did.Number, did.ID, nil)
		if cdr, err := h.deps.DB.CDRs.GetByCallSID(ctx, callSID); err == nil {
			now := time.Now()
			cdr.Disposition = "no-answer"
			cdr.EndedAt = &now
			h.deps.DB.CDRs.Update(ctx, cdr)
		}
		notifyMissedCall(ctx, h.deps, input.From, did.Number, callSID, "no-answer", false)
	case "sms":
		body := strings.TrimSpace(input.Body)
		if body == "" || !utf8.ValidString(body) || utf8.RuneCountInString(body) > 1600 {
			WriteValidationError(w, "Type a message of 1 to 1600 characters", nil)
			return
		}
		messageSID := simulatorSID("SMSIM")
		didID := did.ID
		message := &models.Message{
			DIDID: &didID, Direction: "inbound", FromNumber: input.From, ToNumber: did.Number,
			Body: body, MediaURLs: []byte("[]"), Status: "received", MessageSID: messageSID, CreatedAt: time.Now(),
		}
		if err := h.deps.DB.Messages.Create(ctx, message); err != nil {
			WriteInternalError(w)
			return
		}
		handleAutomationSMS(ctx, h.deps, input.From, did.Number, body, messageSID)
	default:
		WriteValidationError(w, "Action must be missed_call or sms", nil)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "did_id": did.ID, "business_number": did.Number,
		"automation_connected": automationKeyConfigured(h.deps) && h.deps.Config.AutomationMissedCallURL != "" && h.deps.Config.AutomationSMSURL != "",
		"ai_enabled":           automationAIEnabled(ctx, h.deps),
	})
}
