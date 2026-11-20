package api

import (
	"bytes"
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// n8n automation on the phone's own number.
//
// The SIP server keeps the Twilio number's webhooks so the phone keeps
// working. It forwards missed calls and incoming SMS to n8n, and n8n sends SMS
// back through POST /api/automation/sms. Both directions use the
// X-Leadomi-Key header (GOSIP_AUTOMATION_KEY).

const (
	automationKeyHeader  = "X-Leadomi-Key"
	automationActor      = "Automation (n8n)"
	automationDemoKey    = "automation_demo_mode"
	automationAIKey      = "automation_ai_enabled"
	automationHumanPause = 24 * time.Hour
	// A missed call lets the AI answer that caller for this long; each
	// forwarded reply extends it. Other texts (e.g. outreach replies) never
	// reach the AI.
	automationCallSession = 24 * time.Hour
	automationMinKeyBytes = 32
)

var automationClient = &http.Client{Timeout: 15 * time.Second}

// phoneNumberFor returns the number chosen for the phone in the console, or
// the GOSIP_OUTBOUND_CALLER_ID default.
func phoneNumberFor(ctx context.Context, deps *Dependencies) string {
	if chosen, err := deps.DB.Config.Get(ctx, phoneNumberKey); err == nil && e164Destination.MatchString(chosen) {
		return chosen
	}
	if deps.Config == nil {
		return ""
	}
	return deps.Config.OutboundCallerID
}

func automationKeyConfigured(deps *Dependencies) bool {
	return deps.Config != nil && len(deps.Config.AutomationKey) >= automationMinKeyBytes
}

func automationSetting(ctx context.Context, deps *Dependencies, key string) bool {
	value, err := deps.DB.Config.Get(ctx, key)
	return err == nil && value == "true"
}

// automationDemoMode: calls are rejected as busy and handed to n8n at once.
func automationDemoMode(ctx context.Context, deps *Dependencies) bool {
	return automationKeyConfigured(deps) && deps.Config.AutomationMissedCallURL != "" && automationSetting(ctx, deps, automationDemoKey)
}

// automationAIEnabled: incoming SMS are forwarded to the n8n assistant.
func automationAIEnabled(ctx context.Context, deps *Dependencies) bool {
	return automationKeyConfigured(deps) && deps.Config.AutomationSMSURL != "" && automationSetting(ctx, deps, automationAIKey)
}

type automationContact struct {
	OptedOut     bool
	PausedUntil  *time.Time
	SessionUntil *time.Time
}

func getAutomationContact(ctx context.Context, deps *Dependencies, number string) automationContact {
	var c automationContact
	var paused, session sql.NullTime
	deps.DB.Conn().QueryRowContext(ctx, "SELECT opted_out,ai_paused_until,ai_session_until FROM automation_contacts WHERE number=?", number).Scan(&c.OptedOut, &paused, &session)
	if paused.Valid {
		c.PausedUntil = &paused.Time
	}
	if session.Valid {
		c.SessionUntil = &session.Time
	}
	return c
}

func setAutomationOptOut(ctx context.Context, deps *Dependencies, number string, optedOut bool) {
	if _, err := deps.DB.Conn().ExecContext(ctx, "INSERT INTO automation_contacts(number,opted_out) VALUES(?,?) ON CONFLICT(number) DO UPDATE SET opted_out=excluded.opted_out,updated_at=CURRENT_TIMESTAMP", number, optedOut); err != nil {
		slog.Error("Could not save SMS opt-out", "error", err)
	}
}

// pauseAutomationAI hands a conversation to a human for automationHumanPause.
func pauseAutomationAI(ctx context.Context, deps *Dependencies, number string) {
	until := time.Now().UTC().Add(automationHumanPause)
	if _, err := deps.DB.Conn().ExecContext(ctx, "INSERT INTO automation_contacts(number,ai_paused_until) VALUES(?,?) ON CONFLICT(number) DO UPDATE SET ai_paused_until=excluded.ai_paused_until,updated_at=CURRENT_TIMESTAMP", number, until); err != nil {
		slog.Error("Could not pause AI for conversation", "error", err)
	}
}

// openAutomationSession lets the AI answer this caller for automationCallSession.
func openAutomationSession(ctx context.Context, deps *Dependencies, number string) {
	until := time.Now().UTC().Add(automationCallSession)
	if _, err := deps.DB.Conn().ExecContext(ctx, "INSERT INTO automation_contacts(number,ai_session_until) VALUES(?,?) ON CONFLICT(number) DO UPDATE SET ai_session_until=excluded.ai_session_until,updated_at=CURRENT_TIMESTAMP", number, until); err != nil {
		slog.Error("Could not open AI session for caller", "error", err)
	}
}

// smsKeyword returns "stop", "start" or "" for UK/US opt-out keywords.
func smsKeyword(body string) string {
	switch strings.ToUpper(strings.Trim(strings.TrimSpace(body), ".!")) {
	// "YES" and "CANCEL" are deliberately excluded: customers use them in
	// booking conversations ("yes, 10am works", "cancel my visit").
	case "STOP", "STOPALL", "UNSUBSCRIBE", "END", "QUIT", "OPTOUT", "OPT OUT":
		return "stop"
	case "START", "UNSTOP":
		return "start"
	}
	return ""
}

// postAutomationEvent sends one event to n8n in the background. The payload
// uses Twilio's field names so existing workflows read body.From / body.Body.
func postAutomationEvent(deps *Dependencies, url string, payload map[string]string) {
	if url == "" || !automationKeyConfigured(deps) {
		return
	}
	data, _ := json.Marshal(payload)
	key := deps.Config.AutomationKey
	safeGo(func() {
		for attempt := 1; attempt <= 3; attempt++ {
			req, err := http.NewRequest("POST", url, bytes.NewReader(data))
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(automationKeyHeader, key)
			res, err := automationClient.Do(req)
			if err == nil {
				io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
				res.Body.Close()
				if res.StatusCode < 300 {
					return
				}
				if res.StatusCode < 500 {
					slog.Error("n8n rejected automation event", "event", payload["Event"], "status", res.StatusCode)
					return
				}
			}
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
		slog.Error("n8n automation event could not be delivered", "event", payload["Event"])
	})
}

var (
	missedCallsMu sync.Mutex
	missedCalls   = map[string]time.Time{}
)

// firstMissedCallEvent reports whether this call has not triggered a text-back
// yet, so Twilio retries never send two messages for one call.
func firstMissedCallEvent(callSID string) bool {
	if callSID == "" {
		return true
	}
	missedCallsMu.Lock()
	defer missedCallsMu.Unlock()
	now := time.Now()
	for sid, at := range missedCalls {
		if now.Sub(at) > time.Hour {
			delete(missedCalls, sid)
		}
	}
	if _, seen := missedCalls[callSID]; seen {
		return false
	}
	missedCalls[callSID] = now
	return true
}

// notifyMissedCall tells n8n about an unanswered call (or a demo-mode call).
func notifyMissedCall(ctx context.Context, deps *Dependencies, from, to, callSID, status string, demo bool) {
	if !automationKeyConfigured(deps) || deps.Config.AutomationMissedCallURL == "" || !e164Destination.MatchString(from) {
		return
	}
	if !firstMissedCallEvent(callSID) {
		return
	}
	if getAutomationContact(ctx, deps, from).OptedOut {
		return
	}
	// Calling is what invites the AI conversation.
	openAutomationSession(ctx, deps, from)
	demoValue := "false"
	if demo {
		demoValue = "true"
	}
	postAutomationEvent(deps, deps.Config.AutomationMissedCallURL, map[string]string{
		"Event": "missed_call", "From": from, "To": to, "CallSid": callSID, "CallStatus": status, "Demo": demoValue,
	})
}

// handleAutomationSMS processes opt-out keywords and forwards the message to
// the n8n assistant only for contacts who called (open session), when AI
// replies are on and no human has taken over. Replies to outreach texts from
// people who never called go to the phone and Messages only.
func handleAutomationSMS(ctx context.Context, deps *Dependencies, from, to, body, messageSID string) {
	if !e164Destination.MatchString(from) {
		return
	}
	switch smsKeyword(body) {
	case "stop":
		setAutomationOptOut(ctx, deps, from, true)
		return
	case "start":
		setAutomationOptOut(ctx, deps, from, false)
		return
	}
	if !automationAIEnabled(ctx, deps) {
		return
	}
	contact := getAutomationContact(ctx, deps, from)
	now := time.Now()
	if contact.OptedOut || contact.SessionUntil == nil || !now.Before(*contact.SessionUntil) ||
		(contact.PausedUntil != nil && now.Before(*contact.PausedUntil)) {
		return
	}
	openAutomationSession(ctx, deps, from) // keep an active conversation going
	postAutomationEvent(deps, deps.Config.AutomationSMSURL, map[string]string{
		"Event": "sms", "From": from, "To": to, "Body": body, "MessageSid": messageSID,
	})
}

// AutomationHandler serves the n8n-facing API and the console settings.
type AutomationHandler struct{ deps *Dependencies }

func NewAutomationHandler(deps *Dependencies) *AutomationHandler {
	return &AutomationHandler{deps: deps}
}

func (h *AutomationHandler) authorized(r *http.Request) bool {
	if !automationKeyConfigured(h.deps) {
		return false
	}
	key := r.Header.Get(automationKeyHeader)
	if key == "" {
		key = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}
	return subtle.ConstantTimeCompare([]byte(key), []byte(h.deps.Config.AutomationKey)) == 1
}

// SendSMS lets n8n send an SMS from the phone's number. It is recorded in
// Messages, rate-limited by the business policy and refused for opted-out
// contacts. Accepts {"to":"+44...","body":"..."}; "To"/"Body" also work.
func (h *AutomationHandler) SendSMS(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		WriteError(w, http.StatusUnauthorized, ErrCodeAuthentication, "Invalid automation key", nil)
		return
	}
	var input struct {
		To      string `json:"to"`
		Body    string `json:"body"`
		ToAlt   string `json:"To"`
		BodyAlt string `json:"Body"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&input) != nil {
		WriteValidationError(w, "Send JSON with to and body", nil)
		return
	}
	if input.To == "" {
		input.To = input.ToAlt
	}
	if input.Body == "" {
		input.Body = input.BodyAlt
	}
	input.To = strings.TrimSpace(input.To)
	if !e164Destination.MatchString(input.To) || strings.TrimSpace(input.Body) == "" || !utf8.ValidString(input.Body) || utf8.RuneCountInString(input.Body) > 1600 {
		WriteValidationError(w, "Use an international +number and a message of 1 to 1600 characters", nil)
		return
	}
	if getAutomationContact(r.Context(), h.deps, input.To).OptedOut {
		WriteError(w, http.StatusForbidden, ErrCodeAuthorization, "This contact replied STOP and has opted out", nil)
		return
	}
	number := phoneNumberFor(r.Context(), h.deps)
	did, err := h.deps.DB.DIDs.GetByNumber(r.Context(), number)
	if err != nil || !did.SMSEnabled {
		WriteError(w, http.StatusServiceUnavailable, ErrCodeBadRequest, "The phone's number is not set up for SMS yet", nil)
		return
	}
	body, _ := json.Marshal(SendMessageRequest{DIDID: did.ID, ToNumber: input.To, Body: input.Body})
	r.Body = io.NopCloser(bytes.NewReader(body))
	r = r.WithContext(context.WithValue(r.Context(), messageActorKey{}, automationActor))
	NewMessageHandler(h.deps).Send(w, r)
}

type automationSettings struct {
	DemoMode   bool `json:"demo_mode"`
	AIEnabled  bool `json:"ai_enabled"`
	KeySet     bool `json:"key_set"`
	MissedCall bool `json:"missed_call_url_set"`
	SMS        bool `json:"sms_url_set"`
}

// Settings reads (GET) or changes (PUT {demo_mode, ai_enabled}) the toggles.
func (h *AutomationHandler) Settings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodPut {
		var input struct {
			DemoMode  *bool `json:"demo_mode"`
			AIEnabled *bool `json:"ai_enabled"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&input) != nil {
			WriteValidationError(w, "Invalid settings", nil)
			return
		}
		for key, value := range map[string]*bool{automationDemoKey: input.DemoMode, automationAIKey: input.AIEnabled} {
			if value == nil {
				continue
			}
			text := "false"
			if *value {
				text = "true"
			}
			if err := h.deps.DB.Config.Set(ctx, key, text); err != nil {
				WriteInternalError(w)
				return
			}
		}
	}
	c := h.deps.Config
	WriteJSON(w, http.StatusOK, automationSettings{
		DemoMode:   automationSetting(ctx, h.deps, automationDemoKey),
		AIEnabled:  automationSetting(ctx, h.deps, automationAIKey),
		KeySet:     automationKeyConfigured(h.deps),
		MissedCall: c != nil && c.AutomationMissedCallURL != "",
		SMS:        c != nil && c.AutomationSMSURL != "",
	})
}
