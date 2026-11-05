package api

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"
)

// PBXHandler accepts messages only from the authenticated SIP bridge.
// The sender DID is selected server-side, never supplied by the SIP client.
type PBXHandler struct{ deps *Dependencies }

var pbxNumber = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

func (h *PBXHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	secret := h.deps.Config.PBXSecret
	if secret == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+secret)) != 1 {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	var input struct {
		Username string `json:"username"`
		ToNumber string `json:"to_number"`
		Body     string `json:"body"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&input); err != nil || !pbxNumber.MatchString(input.ToNumber) || strings.TrimSpace(input.Body) == "" || !utf8.ValidString(input.Body) || utf8.RuneCountInString(input.Body) > 1600 {
		WriteValidationError(w, "Use an international +number and a message of 1 to 1600 characters", nil)
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		WriteValidationError(w, "Invalid request body", nil)
		return
	}
	if _, err := h.deps.DB.Devices.GetByUsername(r.Context(), input.Username); err != nil {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	number, err := deviceOutboundNumber(r.Context(), h.deps, input.Username)
	if err != nil {
		WriteError(w, http.StatusServiceUnavailable, ErrCodeBadRequest, "SMS sender is unavailable", nil)
		return
	}
	did, err := h.deps.DB.DIDs.GetByNumber(r.Context(), number)
	if err != nil || !did.SMSEnabled || h.deps.Twilio == nil {
		WriteError(w, http.StatusServiceUnavailable, ErrCodeBadRequest, "SMS sender is unavailable", nil)
		return
	}
	body, _ := json.Marshal(SendMessageRequest{DIDID: did.ID, ToNumber: input.ToNumber, Body: input.Body})
	r.Body = io.NopCloser(bytes.NewReader(body))
	r = r.WithContext(context.WithValue(r.Context(), messageActorKey{}, input.Username))
	NewMessageHandler(h.deps).Send(w, r)
}
