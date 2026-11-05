package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rohitkumar-co-in/leadomi-sip/internal/db"
)

type excludedNumber struct {
	Number    string `json:"number"`
	Reason    string `json:"reason"`
	CreatedAt string `json:"created_at"`
}

func numberExclusions(ctx context.Context, database *db.DB) ([]excludedNumber, error) {
	raw, err := database.Config.Get(ctx, "business_excluded_numbers")
	if err == db.ErrConfigNotFound {
		return []excludedNumber{}, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []excludedNumber
	if json.Unmarshal([]byte(raw), &entries) != nil {
		return nil, fmt.Errorf("number protection could not be loaded")
	}
	for _, entry := range entries {
		if !e164Destination.MatchString(entry.Number) {
			return nil, fmt.Errorf("invalid number protection")
		}
	}
	if entries == nil {
		entries = []excludedNumber{}
	}
	return entries, nil
}

func checkNumberProtection(ctx context.Context, database *db.DB, number string) error {
	entries, err := numberExclusions(ctx, database)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Number == number {
			return fmt.Errorf("This number is excluded from configuration. Remove its protection in Excluded numbers before changing it")
		}
	}
	return nil
}

func (h *BusinessHandler) Exclusions(w http.ResponseWriter, r *http.Request) {
	businessMu.Lock()
	defer businessMu.Unlock()
	entries, err := numberExclusions(r.Context(), h.deps.DB)
	if err != nil {
		WriteError(w, 503, "PROTECTION_UNAVAILABLE", err.Error(), nil)
		return
	}
	if r.Method == "GET" {
		WriteJSON(w, 200, map[string]interface{}{"data": entries})
		return
	}
	number := r.URL.Query().Get("number")
	var req excludedNumber
	if r.Method == "PUT" {
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&req) != nil {
			WriteValidationError(w, "Enter a number and optional reason", nil)
			return
		}
		number = strings.TrimSpace(req.Number)
	}
	if !e164Destination.MatchString(number) || len(req.Reason) > 256 {
		WriteValidationError(w, "Use +country-code-number and a reason up to 256 characters", nil)
		return
	}
	result := []excludedNumber{}
	for _, entry := range entries {
		if entry.Number != number {
			result = append(result, entry)
		} else {
			req.CreatedAt = entry.CreatedAt
		}
	}
	action := "unprotect_number"
	if r.Method == "PUT" {
		if len(result) >= 1000 {
			WriteValidationError(w, "Too many excluded numbers", nil)
			return
		}
		if req.CreatedAt == "" {
			req.CreatedAt = time.Now().UTC().Format(time.RFC3339)
		}
		req.Number = number
		req.Reason = strings.TrimSpace(req.Reason)
		result = append(result, req)
		action = "protect_number"
	}
	raw, _ := json.Marshal(result)
	if err = h.deps.DB.Config.Set(r.Context(), "business_excluded_numbers", string(raw)); err != nil {
		WriteInternalError(w)
		return
	}
	h.audit(r.Context(), getUserIDFromContext(r.Context()), action, number, "saved")
	WriteJSON(w, 200, map[string]interface{}{"data": result})
}

// Delete archives the local record after confirmed credential revocation. IDs
// and history are retained; the Twilio number and its callbacks are untouched.
func (h *BusinessHandler) Delete(w http.ResponseWriter, r *http.Request) {
	http.NewResponseController(w).SetWriteDeadline(time.Now().Add(120 * time.Second))
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	businessMu.Lock()
	defer businessMu.Unlock()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid SIP user", nil)
		return
	}
	device, err := h.deps.DB.Devices.GetByID(ctx, id)
	if err != nil {
		WriteNotFoundError(w, "SIP user")
		return
	}
	var number, state string
	var enabled bool
	if err = h.deps.DB.Conn().QueryRowContext(ctx, "SELECT number,state,enabled FROM sip_accounts WHERE device_id=?", id).Scan(&number, &state, &enabled); err != nil {
		WriteNotFoundError(w, "SIP account")
		return
	}
	if state == "deleted" {
		WriteJSON(w, 200, map[string]string{"state": "deleted"})
		return
	}
	if err = checkNumberProtection(ctx, h.deps.DB, number); err != nil {
		WriteError(w, 409, "NUMBER_PROTECTED", err.Error(), nil)
		return
	}
	if enabled || state != "disabled" {
		WriteError(w, 409, "DISABLE_REQUIRED", "Disable this SIP user before deleting it. This prevents leaving an active phone or provider credential", nil)
		return
	}
	list, err := h.credentialList(ctx)
	if err == nil {
		var credential string
		credential, err = h.findCredential(ctx, list, device.Username)
		if err == nil && credential != "" {
			err = h.twilio(ctx, "DELETE", "/SIP/CredentialLists/"+list+"/Credentials/"+credential+".json", nil, nil)
		}
	}
	if err != nil {
		WriteError(w, 502, "REVOCATION_PENDING", "Deletion paused because Twilio credential revocation could not be verified. Retry when Twilio is available", nil)
		return
	}
	tx, err := h.deps.DB.Conn().BeginTx(ctx, nil)
	if err != nil {
		WriteInternalError(w)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "UPDATE sip_accounts SET number='',credential_sid='',enabled=0,state='deleted',updated_at=CURRENT_TIMESTAMP WHERE device_id=?", id); err != nil {
		WriteInternalError(w)
		return
	}
	if _, err = tx.ExecContext(ctx, "UPDATE devices SET password_hash='' WHERE id=?", id); err != nil {
		WriteInternalError(w)
		return
	}
	if err = tx.Commit(); err != nil {
		WriteInternalError(w)
		return
	}
	h.audit(ctx, getUserIDFromContext(r.Context()), "delete_sip_user", device.Username, "deleted_history_preserved")
	WriteJSON(w, 200, map[string]string{"state": "deleted"})
}
