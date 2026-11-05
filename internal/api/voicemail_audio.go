package api

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

func recordingURL(raw, account string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "api.twilio.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	prefix := "/2010-04-01/Accounts/" + account + "/Recordings/"
	if !strings.HasPrefix(u.Path, prefix) {
		return "", false
	}
	recording := strings.TrimPrefix(u.Path, prefix)
	recording = strings.TrimSuffix(strings.TrimSuffix(recording, ".mp3"), ".wav")
	if !regexp.MustCompile(`^RE[a-fA-F0-9]{32}$`).MatchString(recording) {
		return "", false
	}
	u.Path = prefix + recording + ".mp3"
	return u.String(), true
}

// Audio authenticates server-side rather than exposing the Twilio API token to
// the browser. Stored URLs must belong to this account's Twilio Recordings API.
func (h *VoicemailHandler) Audio(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid voicemail ID", nil)
		return
	}
	vm, err := h.deps.DB.Voicemails.GetByID(r.Context(), id)
	if err != nil {
		WriteNotFoundError(w, "Voicemail")
		return
	}
	sid, token := h.deps.Config.TwilioCredentials()
	target, ok := recordingURL(vm.AudioURL, sid)
	if !ok || token == "" {
		WriteError(w, 404, "RECORDING_UNAVAILABLE", "Recording is unavailable", nil)
		return
	}
	request, _ := http.NewRequestWithContext(r.Context(), "GET", target, nil)
	request.SetBasicAuth(sid, token)
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(next *http.Request, previous []*http.Request) error {
		if len(previous) > 3 || next.URL.Scheme != "https" || next.URL.Host != "api.twilio.com" {
			return fmt.Errorf("Recording redirect rejected")
		}
		return nil
	}}
	response, err := client.Do(request)
	if err != nil {
		WriteError(w, 502, "RECORDING_UNAVAILABLE", "Provider recording could not be fetched", nil)
		return
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		WriteError(w, 502, "RECORDING_UNAVAILABLE", "Provider recording is unavailable", nil)
		return
	}
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "private, no-store")
	io.Copy(w, io.LimitReader(response.Body, 25<<20))
}
