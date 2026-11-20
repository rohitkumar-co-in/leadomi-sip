package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/rohitkumar-co-in/leadomi-sip/internal/db"
	"github.com/rohitkumar-co-in/leadomi-sip/pkg/sip"
)

// The single phone user is defined by the environment instead of the console:
//
//	PBX_TWILIO_USER          SIP username (shares the bootstrap Twilio credential)
//	GOSIP_OUTBOUND_CALLER_ID assigned Twilio number
//	GOSIP_PHONE_PASSWORD     phone (Linphone) password
//	GOSIP_PHONE_NAME         optional display name
//
// A ready account with the same number is never re-provisioned: restarts do not
// write to Twilio. A changed phone password only updates the local digest.

// envPhoneUserConfigured reports whether the environment defines the phone user.
func (h *BusinessHandler) envPhoneUserConfigured() bool {
	c := h.deps.Config
	return c != nil && c.TwilioBootstrapUser != "" && c.OutboundCallerID != "" && c.PhonePassword != ""
}

// EnsureEnvPhoneUser creates, repairs or updates the environment-defined phone
// user. It returns nil without changes when the environment does not define one.
func (h *BusinessHandler) EnsureEnvPhoneUser(ctx context.Context) error {
	if !h.envPhoneUserConfigured() {
		return nil
	}
	c := h.deps.Config
	username, number, password := c.TwilioBootstrapUser, c.OutboundCallerID, c.PhonePassword
	name := strings.TrimSpace(c.PhoneName)
	if name == "" {
		name = username
	}
	if !validBusinessUsername(username) {
		return fmt.Errorf("PBX_TWILIO_USER is not a valid SIP username")
	}
	if !e164Destination.MatchString(number) {
		return fmt.Errorf("GOSIP_OUTBOUND_CALLER_ID must be an E.164 number such as +447700900000")
	}
	if !validPhonePassword(password) {
		return fmt.Errorf("GOSIP_PHONE_PASSWORD must be 12-72 printable characters without spaces")
	}
	hash := sip.GenerateHA1(username, "gosip", password)

	device, err := h.deps.DB.Devices.GetByUsername(ctx, username)
	if err != nil && err != db.ErrDeviceNotFound {
		return err
	}
	if device != nil {
		var current, state string
		var enabled bool
		h.deps.DB.Conn().QueryRowContext(ctx, "SELECT number,state,enabled FROM sip_accounts WHERE device_id=?", device.ID).Scan(&current, &state, &enabled)
		if state == "deleted" {
			return fmt.Errorf("SIP user %s was deleted and cannot be reused", username)
		}
		if state == "ready" && enabled && current == number {
			if device.PasswordHash != hash || device.Name != name {
				if _, err = h.deps.DB.Conn().ExecContext(ctx, "UPDATE devices SET name=?,password_hash=? WHERE id=?", name, hash, device.ID); err != nil {
					return err
				}
				h.audit(ctx, 0, "env_phone_user", username, "password_updated")
			}
			return nil
		}
	}

	// Provisioning is needed: reuse the console's checks (ownership, number
	// protection, Messaging Service blocking) by running the same handler.
	numbers, err := h.numbers(ctx)
	if err != nil {
		return err
	}
	var owned *businessNumber
	for i := range numbers {
		if numbers[i].Number == number {
			owned = &numbers[i]
			break
		}
	}
	if owned == nil {
		return fmt.Errorf("%s is not a number owned by this Twilio account", number)
	}
	review, err := h.reviewNumber(ctx, *owned)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(businessRequest{Name: name, Username: username, Number: number, Password: password, Reset: true, NumberReview: review.Fingerprint})
	method, path := "POST", "/accounts"
	if device != nil {
		method, path = "PUT", fmt.Sprintf("/accounts/%d", device.ID)
	}
	router := chi.NewRouter()
	router.Post("/accounts", h.Provision)
	router.Put("/accounts/{id}", h.Provision)
	req := httptest.NewRequest(method, path, bytes.NewReader(body)).WithContext(ctx)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		var failure struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		json.Unmarshal(rec.Body.Bytes(), &failure)
		return fmt.Errorf("provisioning %s failed (%d): %s", username, rec.Code, failure.Error.Message)
	}
	slog.Info("Phone user provisioned from environment", "username", username, "number", number)
	return nil
}

// syncEnvPhoneUser retries until the environment-defined user is ready, since
// the PBX and Twilio may not be reachable immediately after startup.
func (h *BusinessHandler) syncEnvPhoneUser() {
	if !h.envPhoneUserConfigured() {
		return
	}
	for attempt := 1; ; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		err := h.EnsureEnvPhoneUser(ctx)
		cancel()
		if err == nil {
			return
		}
		slog.Error("Phone user from environment is not ready yet", "attempt", attempt, "error", err.Error())
		delay := time.Duration(attempt) * 30 * time.Second
		if delay > 10*time.Minute {
			delay = 10 * time.Minute
		}
		time.Sleep(delay)
	}
}
