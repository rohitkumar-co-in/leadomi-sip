package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rohitkumar-co-in/leadomi-sip/internal/config"
)

const testAutomationKey = "test-automation-key-0123456789abcdef"

type n8nRecorder struct {
	mu     sync.Mutex
	events []map[string]string
	keys   []string
}

func (n *n8nRecorder) handler(w http.ResponseWriter, r *http.Request) {
	var payload map[string]string
	json.NewDecoder(r.Body).Decode(&payload)
	n.mu.Lock()
	n.events = append(n.events, payload)
	n.keys = append(n.keys, r.Header.Get(automationKeyHeader))
	n.mu.Unlock()
}

// waitEvents waits for background event delivery.
func (n *n8nRecorder) waitEvents(count int) []map[string]string {
	for i := 0; i < 100; i++ {
		n.mu.Lock()
		if len(n.events) >= count {
			events := append([]map[string]string(nil), n.events...)
			n.mu.Unlock()
			return events
		}
		n.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]map[string]string(nil), n.events...)
}

func setupAutomation(t *testing.T) (*testSetup, *Dependencies, *n8nRecorder) {
	setup := setupTestAPI(t)
	recorder := &n8nRecorder{}
	server := httptest.NewServer(http.HandlerFunc(recorder.handler))
	t.Cleanup(server.Close)
	deps := &Dependencies{DB: setup.DB, Twilio: setup.Twilio, Config: &config.Config{
		AutomationKey: testAutomationKey, AutomationMissedCallURL: server.URL + "/missed-call",
		AutomationSMSURL: server.URL + "/sms-reply", OutboundCallerID: "+447782226484",
	}}
	createTestDID(t, setup.DB, "+447782226484")
	return setup, deps, recorder
}

func TestAutomationSendSMS(t *testing.T) {
	setup, deps, _ := setupAutomation(t)
	h := NewAutomationHandler(deps)
	send := func(key, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/automation/sms", strings.NewReader(body))
		if key != "" {
			req.Header.Set(automationKeyHeader, key)
		}
		rec := httptest.NewRecorder()
		h.SendSMS(rec, req)
		return rec
	}
	assertStatus(t, send("", `{"to":"+447700900001","body":"hi"}`), 401)
	assertStatus(t, send("wrong", `{"to":"+447700900001","body":"hi"}`), 401)
	assertStatus(t, send(testAutomationKey, `{"to":"07700900001","body":"hi"}`), 400)
	// n8n's Twilio-style field names are accepted too.
	assertStatus(t, send(testAutomationKey, `{"To":"+447700900001","Body":"Sorry we missed your call"}`), 202)
	var from, actor string
	setup.DB.Conn().QueryRow("SELECT m.from_number,COALESCE(a.actor,'') FROM messages m LEFT JOIN activity_actors a ON a.kind='sms' AND a.record_id=m.id WHERE m.to_number='+447700900001'").Scan(&from, &actor)
	if from != "+447782226484" || actor != automationActor {
		t.Fatalf("Automation SMS sent from %q by %q", from, actor)
	}
	if c := getAutomationContact(context.Background(), deps, "+447700900001"); c.PausedUntil != nil {
		t.Fatal("An automation message paused the AI")
	}
	setAutomationOptOut(context.Background(), deps, "+447700900002", true)
	assertStatus(t, send(testAutomationKey, `{"to":"+447700900002","body":"offer"}`), 403)
}

func TestAutomationSMSForwardingAndHandover(t *testing.T) {
	setup, deps, recorder := setupAutomation(t)
	ctx := context.Background()
	customer := "+447700900003"
	// AI off: nothing is forwarded.
	handleAutomationSMS(ctx, deps, customer, "+447782226484", "Hello", "SM1")
	setup.DB.Config.Set(ctx, automationAIKey, "true")
	// A reply from someone who never called (e.g. to outreach) never reaches the AI.
	handleAutomationSMS(ctx, deps, customer, "+447782226484", "Who is this?", "SM1b")
	time.Sleep(100 * time.Millisecond)
	if events := recorder.waitEvents(0); len(events) != 0 {
		t.Fatalf("Outreach reply reached the AI: %v", events)
	}
	// Calling opens the AI conversation for that number.
	setup.DB.Config.Set(ctx, automationDemoKey, "false")
	notifyMissedCall(ctx, deps, customer, "+447782226484", "CA-session", "no-answer", false)
	recorder.waitEvents(1)
	handleAutomationSMS(ctx, deps, customer, "+447782226484", "My car needs an MOT", "SM2")
	events := recorder.waitEvents(2)
	if len(events) != 2 || events[1]["From"] != customer || events[1]["Body"] != "My car needs an MOT" || events[1]["Event"] != "sms" || recorder.keys[1] != testAutomationKey {
		t.Fatalf("Caller's SMS was not forwarded to n8n with Twilio fields and key: %v", events)
	}
	// "Yes" is part of a booking conversation, not an opt-in keyword.
	if smsKeyword("Yes") != "" || smsKeyword("cancel") != "" || smsKeyword(" stop ") != "stop" {
		t.Fatal("Opt-out keyword detection is wrong")
	}
	// A human reply from the phone pauses the AI for that contact.
	did, _ := setup.DB.DIDs.GetByNumber(ctx, "+447782226484")
	body, _ := json.Marshal(SendMessageRequest{DIDID: did.ID, ToNumber: customer, Body: "I'll call you back"})
	req := httptest.NewRequest("POST", "/api/messages", strings.NewReader(string(body)))
	req = req.WithContext(context.WithValue(req.Context(), messageActorKey{}, "leadomi"))
	rec := httptest.NewRecorder()
	NewMessageHandler(deps).Send(rec, req)
	assertStatus(t, rec, 202)
	handleAutomationSMS(ctx, deps, customer, "+447782226484", "Thanks", "SM3")
	// STOP opts out and is never forwarded.
	handleAutomationSMS(ctx, deps, "+447700900004", "+447782226484", "STOP", "SM4")
	time.Sleep(100 * time.Millisecond)
	if events = recorder.waitEvents(2); len(events) != 2 {
		t.Fatalf("Paused or STOP conversations reached the AI: %v", events)
	}
	if !getAutomationContact(ctx, deps, "+447700900004").OptedOut {
		t.Fatal("STOP was not recorded")
	}
	handleAutomationSMS(ctx, deps, "+447700900004", "+447782226484", "START", "SM5")
	if getAutomationContact(ctx, deps, "+447700900004").OptedOut {
		t.Fatal("START did not opt back in")
	}
}

func TestAutomationMissedCall(t *testing.T) {
	setup, deps, recorder := setupAutomation(t)
	ctx := context.Background()
	if automationDemoMode(ctx, deps) {
		t.Fatal("Demo mode must be off by default")
	}
	setup.DB.Config.Set(ctx, automationDemoKey, "true")
	if !automationDemoMode(ctx, deps) {
		t.Fatal("Demo mode toggle was ignored")
	}
	notifyMissedCall(ctx, deps, "+447700900005", "+447782226484", "CA1", "no-answer", false)
	notifyMissedCall(ctx, deps, "+447700900005", "+447782226484", "CA1", "no-answer", false) // Twilio retry
	notifyMissedCall(ctx, deps, "anonymous", "+447782226484", "CA2", "no-answer", false)
	setAutomationOptOut(ctx, deps, "+447700900006", true)
	notifyMissedCall(ctx, deps, "+447700900006", "+447782226484", "CA3", "busy", false)
	time.Sleep(100 * time.Millisecond)
	events := recorder.waitEvents(1)
	if len(events) != 1 || events[0]["Event"] != "missed_call" || events[0]["From"] != "+447700900005" || events[0]["CallSid"] != "CA1" {
		t.Fatalf("Unexpected missed-call events: %v", events)
	}
	// Without a key nothing leaves the server.
	deps.Config.AutomationKey = ""
	if automationDemoMode(ctx, deps) || automationAIEnabled(ctx, deps) {
		t.Fatal("Automation ran without a key")
	}
}

func TestAutomationSessionEndAndTurnCap(t *testing.T) {
	setup, deps, recorder := setupAutomation(t)
	ctx := context.Background()
	setup.DB.Config.Set(ctx, automationAIKey, "true")
	caller := "+447700900301"
	notifyMissedCall(ctx, deps, caller, "+447782226484", "CA-cap", "no-answer", false)
	recorder.waitEvents(1)
	// The AI answers at most automationMaxAITurns messages per conversation.
	for i := 0; i < automationMaxAITurns+3; i++ {
		handleAutomationSMS(ctx, deps, caller, "+447782226484", "hello", "SM-cap")
	}
	time.Sleep(150 * time.Millisecond)
	if events := recorder.waitEvents(1 + automationMaxAITurns); len(events) != 1+automationMaxAITurns {
		t.Fatalf("expected %d AI events, got %d", automationMaxAITurns, len(events)-1)
	}
	// A new missed call starts a fresh conversation.
	notifyMissedCall(ctx, deps, caller, "+447782226484", "CA-cap-2", "no-answer", false)
	handleAutomationSMS(ctx, deps, caller, "+447782226484", "back again", "SM-cap-2")
	if events := recorder.waitEvents(3 + automationMaxAITurns); len(events) != 3+automationMaxAITurns {
		t.Fatalf("new call did not reset the conversation: %d events", len(events))
	}
	// n8n ends the session after a booking: later texts go to people only.
	end := func(key, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/automation/session/end", strings.NewReader(body))
		req.Header.Set(automationKeyHeader, key)
		rec := httptest.NewRecorder()
		NewAutomationHandler(deps).EndSession(rec, req)
		return rec
	}
	assertStatus(t, end("wrong", `{"number":"`+caller+`"}`), 401)
	assertStatus(t, end(testAutomationKey, `{"From":"`+caller+`"}`), 200)
	handleAutomationSMS(ctx, deps, caller, "+447782226484", "thanks!", "SM-cap-3")
	time.Sleep(100 * time.Millisecond)
	if events := recorder.waitEvents(3 + automationMaxAITurns); len(events) != 3+automationMaxAITurns {
		t.Fatalf("AI replied after the session ended: %d events", len(events))
	}
}
