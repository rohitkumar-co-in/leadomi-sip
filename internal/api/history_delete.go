package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rohitkumar-co-in/leadomi-sip/internal/db"
)

type twilioHTTPError struct{ Status int }

func (e *twilioHTTPError) Error() string {
	return fmt.Sprintf("Twilio rejected the request (HTTP %d)", e.Status)
}
func providerAlreadyDeleted(err error) bool {
	var response *twilioHTTPError
	return errors.As(err, &response) && response.Status == http.StatusNotFound
}

// DeleteActivity removes only the selected local record and, when explicitly
// confirmed, its matching Twilio resource. Provider failure preserves local data.
func (h *BusinessHandler) DeleteActivity(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil || user.Role != "admin" {
		WriteError(w, 403, ErrCodeAuthorization, "Only administrators can delete history", nil)
		return
	}
	kind := chi.URLParam(r, "kind")
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 || (kind != "sms" && kind != "call") {
		WriteValidationError(w, "Invalid history record", nil)
		return
	}
	var request struct {
		Scope   string `json:"scope"`
		Confirm bool   `json:"confirm"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&request) != nil || !request.Confirm || (request.Scope != "local" && request.Scope != "both") {
		WriteValidationError(w, "Confirm deletion and choose dashboard only or dashboard and Twilio", nil)
		return
	}
	var sid, status, path string
	if kind == "sms" {
		message, err := h.deps.DB.Messages.GetByID(r.Context(), id)
		if errors.Is(err, db.ErrMessageNotFound) {
			WriteNotFoundError(w, "Message")
			return
		}
		if err != nil {
			WriteInternalError(w)
			return
		}
		sid, status = message.MessageSID, message.Status
		path = "/Messages/" + sid + ".json"
	} else {
		call, err := h.deps.DB.CDRs.GetByID(r.Context(), id)
		if errors.Is(err, db.ErrCDRNotFound) {
			WriteNotFoundError(w, "Call")
			return
		}
		if err != nil {
			WriteInternalError(w)
			return
		}
		sid, status = call.CallSID, call.Disposition
		path = "/Calls/" + sid + ".json"
		// Twilio ends every call within 4 hours; a record still "active" after
		// that only missed its final status update.
		if !historyTerminal(kind, status) && !call.StartedAt.IsZero() && time.Since(call.StartedAt) > 4*time.Hour {
			status = "completed"
		}
	}
	if !historyTerminal(kind, status) {
		WriteError(w, 409, "HISTORY_ACTIVE", "Wait until this call or message has finished before deleting it", nil)
		return
	}
	if request.Scope == "both" {
		valid := resourceSID.MatchString(sid) && ((kind == "call" && sid[:2] == "CA") || (kind == "sms" && (sid[:2] == "SM" || sid[:2] == "MM")))
		if !valid {
			WriteError(w, 409, "NO_PROVIDER_RECORD", "This record has no valid Twilio identifier. Use dashboard-only deletion", nil)
			return
		}
		var provider struct {
			SID    string `json:"sid"`
			Status string `json:"status"`
		}
		err = h.twilio(r.Context(), "GET", path, nil, &provider)
		if err != nil && !providerAlreadyDeleted(err) {
			WriteError(w, 502, "TWILIO_ERROR", "Twilio could not verify the record. Dashboard history was kept", nil)
			return
		}
		if err == nil {
			if provider.SID != sid || !historyTerminal(kind, provider.Status) {
				WriteError(w, 409, "HISTORY_ACTIVE", "Twilio reports that this record is still active or cannot be verified", nil)
				return
			}
			err = h.twilio(r.Context(), "DELETE", path, nil, nil)
			if err != nil && !providerAlreadyDeleted(err) {
				h.audit(r.Context(), user.ID, "history_delete", kind+":"+strconv.FormatInt(id, 10), "provider_failed")
				WriteError(w, 502, "TWILIO_ERROR", "Twilio deletion failed. Dashboard history was kept", nil)
				return
			}
		}
	}
	if kind == "sms" {
		err = h.deps.DB.Messages.Delete(r.Context(), id)
	} else {
		err = h.deps.DB.CDRs.Delete(r.Context(), id)
	}
	if err != nil {
		WriteInternalError(w)
		return
	}
	h.audit(r.Context(), user.ID, "history_delete", kind+":"+strconv.FormatInt(id, 10), request.Scope)
	WriteJSON(w, 200, map[string]string{"message": "History record deleted", "scope": request.Scope})
}

func historyTerminal(kind, status string) bool {
	if kind == "call" {
		switch status {
		case "answered", "voicemail", "missed", "blocked", "busy", "failed", "completed", "no-answer", "canceled":
			return true
		}
	}
	if kind == "sms" {
		switch status {
		case "received", "delivered", "undelivered", "failed", "canceled", "read", "sent", "simulated":
			return true
		}
	}
	return false
}
