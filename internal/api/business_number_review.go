package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

type numberReview struct {
	Number      string   `json:"number"`
	Connections []string `json:"connections"`
	Fingerprint string   `json:"fingerprint"`
	Blocked     bool     `json:"blocked"`
}

func validPhonePassword(value string) bool {
	if len(value) < 12 || len(value) > 72 {
		return false
	}
	for _, c := range value {
		if c < 33 || c > 126 {
			return false
		}
	}
	return true
}

// External connections are never changed during inventory, review or startup.
func (h *BusinessHandler) reviewNumber(ctx context.Context, n businessNumber) (numberReview, error) {
	result := numberReview{Number: n.Number, Connections: []string{}}
	if err := checkNumberProtection(ctx, h.deps.DB, n.Number); err != nil {
		result.Blocked = true
		result.Connections = append(result.Connections, err.Error())
		return result, nil
	}
	for _, entry := range []struct{ label, value string }{{"Voice webhook", n.VoiceURL}, {"SMS webhook", n.SMSURL}, {"Call status callback", n.StatusURL}, {"Voice application", n.VoiceApplication}, {"SMS application", n.SMSApplication}} {
		if entry.value != "" {
			result.Connections = append(result.Connections, entry.label+": "+entry.value)
		}
	}
	sid, token := h.deps.Config.TwilioCredentials()
	get := func(path string, v interface{}) error {
		req, _ := http.NewRequestWithContext(ctx, "GET", "https://messaging.twilio.com/v1/"+path, nil)
		req.SetBasicAuth(sid, token)
		res, err := h.client.Do(req)
		if err != nil {
			return fmt.Errorf("Messaging Service connections could not be checked")
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return fmt.Errorf("Messaging Service connections could not be checked")
		}
		return json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(v)
	}
	var services struct {
		Items []struct {
			SID       string `json:"sid"`
			UseNumber bool   `json:"use_inbound_webhook_on_number"`
			URL       string `json:"inbound_request_url"`
		} `json:"services"`
		Meta struct {
			Next string `json:"next_page_url"`
		} `json:"meta"`
	}
	if err := get("Services?PageSize=1000", &services); err != nil {
		return result, err
	}
	if services.Meta.Next != "" {
		return result, fmt.Errorf("Messaging Service inventory is incomplete; assignment is blocked")
	}
	for _, service := range services.Items {
		if !resourceSID.MatchString(service.SID) {
			return result, fmt.Errorf("Invalid Messaging Service")
		}
		var pool struct {
			Items []struct {
				SID string `json:"sid"`
			} `json:"phone_numbers"`
			Meta struct {
				Next string `json:"next_page_url"`
			} `json:"meta"`
		}
		if err := get("Services/"+service.SID+"/PhoneNumbers?PageSize=1000", &pool); err != nil {
			return result, err
		}
		if pool.Meta.Next != "" {
			return result, fmt.Errorf("Messaging Service sender inventory is incomplete; assignment is blocked")
		}
		for _, item := range pool.Items {
			if item.SID == n.SID {
				result.Connections = append(result.Connections, fmt.Sprintf("Messaging Service: %s; number-level inbound routing: %t; handler: %s", service.SID, service.UseNumber, service.URL))
				if !service.UseNumber {
					result.Blocked = true
				}
			}
		}
	}
	// Include local routing in the reviewed state so stale screens cannot replace it.
	rows, err := h.deps.DB.Conn().QueryContext(ctx, "SELECT r.id,r.action_type,r.action_data,r.enabled FROM routes r JOIN dids d ON d.id=r.did_id WHERE d.number=? ORDER BY r.id", n.Number)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	var routes []string
	for rows.Next() {
		var id int64
		var action, data string
		var enabled bool
		if err = rows.Scan(&id, &action, &data, &enabled); err != nil {
			return result, err
		}
		routes = append(routes, fmt.Sprintf("%d:%s:%s:%t", id, action, data, enabled))
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	if len(routes) > 0 {
		result.Connections = append(result.Connections, fmt.Sprintf("Existing local call routing: %d rule(s). Assignment replaces the selected number's rules.", len(routes)))
	}
	sort.Strings(result.Connections)
	raw, _ := json.Marshal(struct {
		Number              businessNumber
		Connections, Routes []string
	}{n, result.Connections, routes})
	sum := sha256.Sum256(raw)
	result.Fingerprint = hex.EncodeToString(sum[:])
	return result, nil
}

func (h *BusinessHandler) ReviewNumber(w http.ResponseWriter, r *http.Request) {
	number := r.URL.Query().Get("number")
	if !e164Destination.MatchString(number) {
		WriteValidationError(w, "Select an existing Twilio number", nil)
		return
	}
	numbers, err := h.numbers(r.Context())
	if err != nil {
		WriteError(w, 502, "TWILIO_ERROR", err.Error(), nil)
		return
	}
	for _, n := range numbers {
		if n.Number == number {
			review, err := h.reviewNumber(r.Context(), n)
			if err != nil {
				WriteError(w, 502, "NUMBER_CHECK_FAILED", err.Error(), nil)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			WriteJSON(w, 200, review)
			return
		}
	}
	WriteNotFoundError(w, "Owned Twilio number")
}

func (h *BusinessHandler) configureNumber(ctx context.Context, sid string) error {
	if !resourceSID.MatchString(sid) || !strings.HasPrefix(h.deps.Config.PublicURL, "https://") {
		return fmt.Errorf("A public HTTPS URL is required")
	}
	// The reviewed provider state was checked before any provisioning writes.
	base := strings.TrimSuffix(h.deps.Config.PublicURL, "/")
	return h.twilio(ctx, "POST", "/IncomingPhoneNumbers/"+sid+".json", map[string][]string{"VoiceUrl": {base + "/api/webhooks/voice/incoming"}, "VoiceMethod": {"POST"}, "SmsUrl": {base + "/api/webhooks/sms/incoming"}, "SmsMethod": {"POST"}, "StatusCallback": {base + "/api/webhooks/voice/status"}, "StatusCallbackMethod": {"POST"}, "VoiceApplicationSid": {""}, "SmsApplicationSid": {""}}, nil)
}
