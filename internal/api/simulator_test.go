package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSimulatorEndToEnd(t *testing.T) {
	setup, deps, recorder := setupAutomation(t)
	ctx := context.Background()
	setup.DB.Config.Set(ctx, automationAIKey, "true")
	var twilioSends int32
	setup.Twilio.SendSMSFunc = func(from, to, body string, mediaURLs []string) (string, error) {
		atomic.AddInt32(&twilioSends, 1)
		return "SM-real", nil
	}
	sim := NewSimulatorHandler(deps)
	simulate := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		sim.Simulate(rec, httptest.NewRequest("POST", "/api/business/simulator", strings.NewReader(body)))
		return rec
	}
	customer := "+447700900123"
	// Only fictional numbers are accepted.
	assertStatus(t, simulate(`{"action":"missed_call","from":"+447911123456"}`), 400)

	// A simulated missed call reaches n8n and opens the AI conversation.
	assertStatus(t, simulate(`{"action":"missed_call","from":"`+customer+`"}`), 200)
	events := recorder.waitEvents(1)
	if len(events) != 1 || events[0]["Event"] != "missed_call" || events[0]["From"] != customer {
		t.Fatalf("Simulated missed call did not reach n8n: %v", events)
	}
	// n8n's text-back to the fictional number is stored, never sent to Twilio.
	auto := httptest.NewRequest("POST", "/api/automation/sms", strings.NewReader(`{"to":"`+customer+`","body":"Hi, thanks for calling Mike Garages!"}`))
	auto.Header.Set(automationKeyHeader, testAutomationKey)
	rec := httptest.NewRecorder()
	NewAutomationHandler(deps).SendSMS(rec, auto)
	assertStatus(t, rec, 202)
	var status string
	for i := 0; i < 100 && status != "simulated"; i++ {
		time.Sleep(10 * time.Millisecond)
		setup.DB.Conn().QueryRow("SELECT status FROM messages WHERE to_number=? AND direction='outbound'", customer).Scan(&status)
	}
	if status != "simulated" || atomic.LoadInt32(&twilioSends) != 0 {
		t.Fatalf("Simulator reply was sent to Twilio (status %q, sends %d)", status, twilioSends)
	}
	// The customer's simulated reply is stored and forwarded to the AI.
	assertStatus(t, simulate(`{"action":"sms","from":"`+customer+`","body":"My car needs an MOT"}`), 200)
	events = recorder.waitEvents(2)
	if len(events) != 2 || events[1]["Event"] != "sms" || events[1]["Body"] != "My car needs an MOT" {
		t.Fatalf("Simulated SMS did not reach the AI: %v", events)
	}
	var inbound int
	setup.DB.Conn().QueryRow("SELECT COUNT(*) FROM messages WHERE from_number=? AND direction='inbound'", customer).Scan(&inbound)
	if inbound != 1 {
		t.Fatal("Simulated SMS was not stored for Messages and the phone")
	}
}
