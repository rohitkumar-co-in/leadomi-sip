package api

// Business provisioning uses existing Twilio numbers only. Phone passwords are
// hashed locally; separate Twilio SIP passwords live in the private PBX volume.
// No Twilio account token or plaintext phone password is stored in SQLite.
import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rohitkumar-co-in/leadomi-sip/internal/db"
	"github.com/rohitkumar-co-in/leadomi-sip/internal/models"
	"github.com/rohitkumar-co-in/leadomi-sip/pkg/sip"
)

var businessMu sync.Mutex
var businessUsername = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{2,63}$`)
var resourceSID = regexp.MustCompile(`^[A-Z]{2}[a-fA-F0-9]{32}$`)

func phoneServer(value string) string {
	if host, _, err := net.SplitHostPort(value); err == nil {
		return host
	}
	return value
}

func validBusinessUsername(username string) bool {
	if !businessUsername.MatchString(username) {
		return false
	}
	name := strings.ToLower(username)
	return name != "global" && name != "tls" && name != "deny" && !strings.HasPrefix(name, "twilio-")
}

type BusinessHandler struct {
	deps    *Dependencies
	client  *http.Client
	apiBase string
}

func newBusinessHandler(deps *Dependencies) *BusinessHandler {
	return &BusinessHandler{deps: deps, client: &http.Client{Timeout: 15 * time.Second}, apiBase: "https://api.twilio.com"}
}
func (h *BusinessHandler) audit(ctx context.Context, actor int64, action, subject, outcome string) {
	h.deps.DB.Conn().ExecContext(ctx, "INSERT INTO business_audit(actor_id,action,subject,outcome) VALUES(?,?,?,?)", actor, action, subject, outcome)
}

// Errors deliberately omit upstream response bodies, URLs and credentials.
func (h *BusinessHandler) twilio(ctx context.Context, method, path string, form url.Values, result interface{}) error {
	sid, token := h.deps.Config.TwilioCredentials()
	if !resourceSID.MatchString(sid) || token == "" {
		return fmt.Errorf("Twilio credentials are unavailable")
	}
	target := h.apiBase + "/2010-04-01/Accounts/" + sid + path
	if strings.HasPrefix(path, "@messaging/") {
		target = strings.TrimSuffix(h.apiBase, "api.twilio.com") + "messaging.twilio.com/v1/" + strings.TrimPrefix(path, "@messaging/")
	}
	req, err := http.NewRequestWithContext(ctx, method, target, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(sid, token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("Twilio request could not complete")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return &twilioHTTPError{Status: res.StatusCode}
	}
	if result != nil {
		return json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(result)
	}
	return nil
}

type businessNumber struct {
	SID              string `json:"sid"`
	Number           string `json:"phone_number"`
	Name             string `json:"friendly_name"`
	VoiceURL         string `json:"voice_url"`
	SMSURL           string `json:"sms_url"`
	StatusURL        string `json:"status_callback"`
	VoiceMethod      string `json:"voice_method"`
	SMSMethod        string `json:"sms_method"`
	StatusMethod     string `json:"status_callback_method"`
	VoiceApplication string `json:"voice_application_sid"`
	SMSApplication   string `json:"sms_application_sid"`
	Capabilities     struct {
		Voice bool `json:"voice"`
		SMS   bool `json:"sms"`
	} `json:"capabilities"`
}

func (h *BusinessHandler) numbers(ctx context.Context) ([]businessNumber, error) {
	var all []businessNumber
	path := "/IncomingPhoneNumbers.json?PageSize=1000"
	for page := 0; page < 20; page++ {
		var list struct {
			Numbers []businessNumber `json:"incoming_phone_numbers"`
			Next    string           `json:"next_page_uri"`
		}
		if err := h.twilio(ctx, http.MethodGet, path, nil, &list); err != nil {
			return nil, err
		}
		all = append(all, list.Numbers...)
		if list.Next == "" {
			return all, nil
		}
		sid, _ := h.deps.Config.TwilioCredentials()
		prefix := "/2010-04-01/Accounts/" + sid
		if !strings.HasPrefix(list.Next, prefix+"/IncomingPhoneNumbers.json?") {
			return nil, fmt.Errorf("Invalid Twilio pagination")
		}
		path = strings.TrimPrefix(list.Next, prefix)
	}
	return nil, fmt.Errorf("Too many Twilio numbers to import")
}
func (h *BusinessHandler) Numbers(w http.ResponseWriter, r *http.Request) {
	numbers, err := h.numbers(r.Context())
	if err != nil {
		WriteError(w, 502, "TWILIO_ERROR", err.Error(), nil)
		return
	}
	WriteJSON(w, 200, map[string]interface{}{"data": numbers})
}
func (h *BusinessHandler) bootstrap(ctx context.Context) error {
	// Import the two previously configured accounts without changing passwords,
	// routing, or Twilio settings. All future assignments are transactional.
	raw, err := h.deps.DB.Config.Get(ctx, "pbx_device_numbers")
	if err == db.ErrConfigNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	var assignments map[string]string
	if err = json.Unmarshal([]byte(raw), &assignments); err != nil {
		return err
	}
	for user, number := range assignments {
		if !e164Destination.MatchString(number) {
			return fmt.Errorf("Invalid existing assignment")
		}
		_, err = h.deps.DB.Conn().ExecContext(ctx, "INSERT OR IGNORE INTO sip_accounts(device_id,number,state) SELECT id,?,'ready' FROM devices WHERE username=?", number, user)
		if err != nil {
			return err
		}
	}
	return nil
}
func (h *BusinessHandler) List(w http.ResponseWriter, r *http.Request) {
	if err := h.bootstrap(r.Context()); err != nil {
		WriteInternalError(w)
		return
	}
	devices, err := h.deps.DB.Devices.List(r.Context(), 1000, 0)
	if err != nil {
		WriteInternalError(w)
		return
	}
	protected, err := numberExclusions(r.Context(), h.deps.DB)
	if err != nil {
		WriteError(w, 503, "PROTECTION_UNAVAILABLE", "Number protection could not be loaded", nil)
		return
	}
	locks := map[string]bool{}
	for _, entry := range protected {
		locks[entry.Number] = true
	}
	online := NewDeviceHandler(h.deps).pbxRegistrations(r)
	result := []map[string]interface{}{}
	for _, d := range devices {
		number, state := "", "pending"
		enabled := true
		err = h.deps.DB.Conn().QueryRowContext(r.Context(), "SELECT number,state,enabled FROM sip_accounts WHERE device_id=?", d.ID).Scan(&number, &state, &enabled)
		if err != nil && err != sql.ErrNoRows {
			WriteInternalError(w)
			return
		}
		if state == "deleted" {
			continue
		}
		result = append(result, map[string]interface{}{"id": d.ID, "name": d.Name, "username": d.Username, "number": number, "state": state, "enabled": enabled, "online": enabled && online[d.ID], "protected": locks[number]})
	}
	w.Header().Set("Cache-Control", "no-store")
	WriteJSON(w, 200, map[string]interface{}{"data": result, "server": phoneServer(h.deps.Config.PBXSIPDomain), "proxy": "sip:" + phoneServer(h.deps.Config.PBXSIPDomain) + ":5061;transport=tls"})
}
func (h *BusinessHandler) credentialList(ctx context.Context) (string, error) {
	domainSID, err := h.deps.DB.Config.Get(ctx, "pbx_twilio_domain_sid")
	if err != nil || !resourceSID.MatchString(domainSID) {
		return "", fmt.Errorf("Twilio SIP domain must be configured by the server administrator")
	}
	listSID, err := h.deps.DB.Config.Get(ctx, "pbx_twilio_credential_list_sid")
	if err != nil || !resourceSID.MatchString(listSID) {
		return "", fmt.Errorf("Twilio credential list must be configured by the server administrator")
	}
	var domain struct {
		Name   string `json:"domain_name"`
		Secure bool   `json:"secure"`
	}
	if err = h.twilio(ctx, "GET", "/SIP/Domains/"+domainSID+".json", nil, &domain); err != nil {
		return "", err
	}
	if domain.Name != h.deps.Config.TwilioSIPDomain || !domain.Secure {
		return "", fmt.Errorf("Twilio SIP domain does not match the secure server configuration")
	}
	for _, kind := range []string{"Calls", "Registrations"} {
		var mappings struct {
			Items []struct {
				SID string `json:"sid"`
			} `json:"contents"`
		}
		if err = h.twilio(ctx, "GET", "/SIP/Domains/"+domainSID+"/Auth/"+kind+"/CredentialListMappings.json?PageSize=1000", nil, &mappings); err != nil {
			return "", err
		}
		found := false
		for _, item := range mappings.Items {
			if item.SID == listSID {
				found = true
			}
		}
		if !found {
			return "", fmt.Errorf("Credential list is not attached to Twilio SIP %s", kind)
		}
	}
	return listSID, nil
}
func (h *BusinessHandler) findCredential(ctx context.Context, listSID, user string) (string, error) {
	var list struct {
		Items []struct {
			SID      string `json:"sid"`
			Username string `json:"username"`
		} `json:"credentials"`
		Next string `json:"next_page_uri"`
	}
	path := "/SIP/CredentialLists/" + listSID + "/Credentials.json?PageSize=1000"
	for page := 0; page < 20; page++ {
		list.Next = ""
		list.Items = nil
		if err := h.twilio(ctx, "GET", path, nil, &list); err != nil {
			return "", err
		}
		for _, item := range list.Items {
			if item.Username == user {
				return item.SID, nil
			}
		}
		if list.Next == "" {
			return "", nil
		}
		sid, _ := h.deps.Config.TwilioCredentials()
		prefix := "/2010-04-01/Accounts/" + sid
		if !strings.HasPrefix(list.Next, prefix+"/SIP/CredentialLists/"+listSID+"/Credentials.json?") {
			return "", fmt.Errorf("Invalid credential pagination")
		}
		path = strings.TrimPrefix(list.Next, prefix)
	}
	return "", fmt.Errorf("Too many credentials")
}
func password() (string, error) {
	var b [18]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "Gs9_" + hex.EncodeToString(b[:]), nil
}
func (h *BusinessHandler) pbxCredential(ctx context.Context, user, password string) error {
	body, _ := json.Marshal(map[string]string{"username": user, "password": password})
	req, err := http.NewRequestWithContext(ctx, "POST", h.deps.Config.PBXURL+"/credentials", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+h.deps.Config.PBXSecret)
	req.Header.Set("Content-Type", "application/json")
	res, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("PBX credential storage is unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("PBX could not save the credential")
	}
	return nil
}

type businessRequest struct {
	Name         string `json:"name"`
	Username     string `json:"username"`
	Number       string `json:"number"`
	Reset        bool   `json:"reset_password"`
	Password     string `json:"password"`
	NumberReview string `json:"number_review"`
}

func (h *BusinessHandler) Provision(w http.ResponseWriter, r *http.Request) {
	// Provisioning makes several provider requests. Extend only this route's
	// write deadline; ordinary API requests retain the server's short timeout.
	http.NewResponseController(w).SetWriteDeadline(time.Now().Add(120 * time.Second))
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	businessMu.Lock()
	defer businessMu.Unlock()
	var req businessRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || strings.TrimSpace(req.Name) == "" || len(req.Name) > 128 || !e164Destination.MatchString(req.Number) {
		WriteValidationError(w, "A name and an existing Twilio number in +country-code format are required", nil)
		return
	}
	if err := checkNumberProtection(r.Context(), h.deps.DB, req.Number); err != nil {
		WriteError(w, 409, "NUMBER_PROTECTED", err.Error(), nil)
		return
	}
	if h.deps.Config.PBXURL == "" {
		WriteError(w, 503, "PBX_UNAVAILABLE", "The PBX is not configured", nil)
		return
	}
	if err := h.bootstrap(r.Context()); err != nil {
		WriteInternalError(w)
		return
	}
	var device *models.Device
	idParam := chi.URLParam(r, "id")
	if idParam != "" {
		id, err := strconv.ParseInt(idParam, 10, 64)
		if err != nil {
			WriteValidationError(w, "Invalid SIP user", nil)
			return
		}
		device, err = h.deps.DB.Devices.GetByID(r.Context(), id)
		if err != nil {
			WriteNotFoundError(w, "SIP user")
			return
		}
		var previousNumber, previousState string
		h.deps.DB.Conn().QueryRowContext(r.Context(), "SELECT number,state FROM sip_accounts WHERE device_id=?", device.ID).Scan(&previousNumber, &previousState)
		if previousState == "deleted" {
			WriteNotFoundError(w, "SIP user")
			return
		}
		if err := checkNumberProtection(r.Context(), h.deps.DB, previousNumber); err != nil {
			WriteError(w, 409, "NUMBER_PROTECTED", err.Error(), nil)
			return
		}
		req.Username = device.Username
	} else if !validBusinessUsername(req.Username) {
		WriteValidationError(w, "Username must start with a letter and contain 3â€“64 letters, digits, underscores or hyphens", nil)
		return
	}
	var assigned int64
	var assignmentEnabled bool
	err := h.deps.DB.Conn().QueryRowContext(r.Context(), "SELECT device_id,enabled FROM sip_accounts WHERE number=?", req.Number).Scan(&assigned, &assignmentEnabled)
	if err != nil && err != sql.ErrNoRows {
		WriteInternalError(w)
		return
	}
	if err == nil && assignmentEnabled && (device == nil || assigned != device.ID) {
		WriteError(w, 409, "NUMBER_ASSIGNED", "That number is already assigned to another SIP user", nil)
		return
	}
	numbers, err := h.numbers(r.Context())
	if err != nil {
		WriteError(w, 502, "TWILIO_ERROR", err.Error(), nil)
		return
	}
	var owned *businessNumber
	for i := range numbers {
		if numbers[i].Number == req.Number {
			owned = &numbers[i]
			break
		}
	}
	if owned == nil || !owned.Capabilities.Voice || !owned.Capabilities.SMS {
		WriteValidationError(w, "Select an owned Twilio number supporting both voice and SMS", nil)
		return
	}
	// A ready user's ordinary edits never touch provider credentials or routing.
	var oldNumber, oldState string
	var oldEnabled bool
	if device != nil {
		h.deps.DB.Conn().QueryRowContext(r.Context(), "SELECT number,state,enabled FROM sip_accounts WHERE device_id=?", device.ID).Scan(&oldNumber, &oldState, &oldEnabled)
	}
	if req.Password != "" && !validPhonePassword(req.Password) {
		WriteValidationError(w, "Phone password must be 12â€“72 printable ASCII characters without spaces", nil)
		return
	}
	if device != nil && oldNumber == req.Number && oldState == "ready" && oldEnabled {
		hash := device.PasswordHash
		phonePassword := req.Password
		if req.Reset && phonePassword == "" {
			phonePassword, err = password()
		}
		if err != nil {
			WriteInternalError(w)
			return
		}
		if phonePassword != "" {
			hash = sip.GenerateHA1(device.Username, "gosip", phonePassword)
		}
		if _, err = h.deps.DB.Conn().ExecContext(r.Context(), "UPDATE devices SET name=?,password_hash=? WHERE id=?", req.Name, hash, device.ID); err != nil {
			WriteInternalError(w)
			return
		}
		h.audit(r.Context(), getUserIDFromContext(r.Context()), "edit_user", device.Username, "connections_preserved")
		w.Header().Set("Cache-Control", "no-store")
		WriteJSON(w, 200, map[string]interface{}{"id": device.ID, "username": device.Username, "number": req.Number, "state": "ready", "password": phonePassword, "proxy": "sip:" + phoneServer(h.deps.Config.PBXSIPDomain) + ":5061;transport=tls", "server": phoneServer(h.deps.Config.PBXSIPDomain)})
		return
	}
	// Review is read-only and must precede every external or local mutation.
	review, err := h.reviewNumber(r.Context(), *owned)
	if err != nil {
		WriteError(w, 502, "NUMBER_CHECK_FAILED", err.Error(), nil)
		return
	}
	if review.Blocked {
		WriteError(w, 409, "NUMBER_CONNECTED", "This number uses a Messaging Service inbound handler. Preserve it or enable number-level inbound routing in Twilio yourself before assigning it", nil)
		return
	}
	if req.NumberReview == "" || req.NumberReview != review.Fingerprint {
		WriteError(w, 409, "NUMBER_REVIEW_REQUIRED", "Review the number's current connections and explicitly confirm this assignment first. Its configuration may have changed", nil)
		return
	}
	listSID, err := h.credentialList(r.Context())
	if err != nil {
		WriteError(w, 502, "TWILIO_ERROR", err.Error(), nil)
		return
	}
	credentialSID, err := h.findCredential(r.Context(), listSID, req.Username)
	if err != nil {
		WriteError(w, 502, "TWILIO_ERROR", err.Error(), nil)
		return
	}
	// Never take over an unrelated username already in the Twilio credential list.
	if device == nil && credentialSID != "" {
		WriteError(w, 409, "USERNAME_EXISTS", "That username already exists in Twilio. Choose a different username", nil)
		return
	}
	phonePassword := req.Password
	if device != nil {
		var existingNumber string
		h.deps.DB.Conn().QueryRowContext(r.Context(), "SELECT number FROM sip_accounts WHERE device_id=?", device.ID).Scan(&existingNumber)
		if existingNumber == "" {
			req.Reset = true
		}
	}
	if (device == nil || req.Reset) && phonePassword == "" {
		phonePassword, err = password()
		if err != nil {
			WriteInternalError(w)
			return
		}
	}
	if device == nil {
		device = &models.Device{Name: req.Name, Username: req.Username, PasswordHash: sip.GenerateHA1(req.Username, "gosip", phonePassword), DeviceType: "linphone"}
		if err = h.deps.DB.Devices.Create(r.Context(), device); err != nil {
			WriteError(w, 409, "USERNAME_EXISTS", "That SIP username already exists", nil)
			return
		}
	}
	// Journal the attempt before external writes. A failed operation stays visible
	// and can be retried; we never present a partial configuration as ready.
	_, err = h.deps.DB.Conn().ExecContext(r.Context(), "INSERT INTO sip_accounts(device_id,state) VALUES(?,'provisioning') ON CONFLICT(device_id) DO UPDATE SET state='provisioning',updated_at=CURRENT_TIMESTAMP", device.ID)
	if err != nil {
		WriteInternalError(w)
		return
	}
	failed := func(e error) {
		h.deps.DB.Conn().ExecContext(context.Background(), "UPDATE sip_accounts SET state='error' WHERE device_id=?", device.ID)
		h.audit(context.Background(), getUserIDFromContext(r.Context()), "provision", device.Username, "error")
		WriteError(w, 502, "PROVISIONING_FAILED", e.Error()+". The account is marked for retry", nil)
	}
	trunkPassword, err := password()
	if err != nil {
		failed(fmt.Errorf("Could not generate SIP credential"))
		return
	}
	path := "/SIP/CredentialLists/" + listSID + "/Credentials"
	form := url.Values{"Password": {trunkPassword}}
	if credentialSID == "" {
		form.Set("Username", device.Username)
		path += ".json"
	} else {
		path += "/" + credentialSID + ".json"
	}
	var credential struct {
		SID string `json:"sid"`
	}
	if err = h.twilio(r.Context(), "POST", path, form, &credential); err != nil {
		failed(err)
		return
	}
	credentialSID = credential.SID
	if !resourceSID.MatchString(credentialSID) {
		failed(fmt.Errorf("Twilio returned an invalid credential"))
		return
	}
	if err = h.pbxCredential(r.Context(), device.Username, trunkPassword); err != nil {
		failed(err)
		return
	}
	if err = h.configureNumber(r.Context(), owned.SID); err != nil {
		failed(err)
		return
	}
	if err = h.assign(r.Context(), device, req.Name, req.Number, owned.SID, credentialSID, phonePassword); err != nil {
		failed(fmt.Errorf("Local number assignment could not be saved"))
		return
	}
	h.audit(r.Context(), getUserIDFromContext(r.Context()), "provision", device.Username, "ready")
	w.Header().Set("Cache-Control", "no-store")
	WriteJSON(w, 200, map[string]interface{}{"id": device.ID, "username": device.Username, "number": req.Number, "state": "ready", "password": phonePassword, "proxy": "sip:" + phoneServer(h.deps.Config.PBXSIPDomain) + ":5061;transport=tls", "server": phoneServer(h.deps.Config.PBXSIPDomain)})
}
func (h *BusinessHandler) assign(ctx context.Context, d *models.Device, name, number, numberSID, credential, phonePassword string) error {
	tx, err := h.deps.DB.Conn().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var oldNumber string
	if err = tx.QueryRowContext(ctx, "SELECT number FROM sip_accounts WHERE device_id=?", d.ID).Scan(&oldNumber); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE sip_accounts SET number='' WHERE number=? AND enabled=0 AND device_id<>?", number, d.ID); err != nil {
		return err
	}
	hash := d.PasswordHash
	if phonePassword != "" {
		hash = sip.GenerateHA1(d.Username, "gosip", phonePassword)
	}
	if _, err = tx.ExecContext(ctx, "UPDATE devices SET name=?,password_hash=? WHERE id=?", name, hash, d.ID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO dids(number,twilio_sid,name,voice_enabled,sms_enabled) VALUES(?,?,?,1,1) ON CONFLICT(number) DO UPDATE SET twilio_sid=excluded.twilio_sid,voice_enabled=1,sms_enabled=1", number, numberSID, name); err != nil {
		return err
	}
	var didID int64
	if err = tx.QueryRowContext(ctx, "SELECT id FROM dids WHERE number=?", number).Scan(&didID); err != nil {
		return err
	}
	// One assigned user owns the default route; explicit advanced routes remain
	// editable, but replacing a number's owner removes prior ring destinations.
	if oldNumber != "" && oldNumber != number {
		if _, err = tx.ExecContext(ctx, "DELETE FROM routes WHERE did_id=(SELECT id FROM dids WHERE number=?)", oldNumber); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM routes WHERE did_id=?", didID); err != nil {
		return err
	}
	action, _ := json.Marshal(map[string]interface{}{"devices": []int64{d.ID}, "timeout": 30, "fallback": "voicemail"})
	if _, err = tx.ExecContext(ctx, "INSERT INTO routes(did_id,priority,name,condition_type,action_type,action_data,enabled) VALUES(?,0,?,'default','ring',?,1)", didID, "Assigned to "+name, string(action)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE sip_accounts SET number=?,credential_sid=?,enabled=1,state='ready',updated_at=CURRENT_TIMESTAMP WHERE device_id=?", number, credential, d.ID); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, "SELECT d.username,a.number FROM sip_accounts a JOIN devices d ON d.id=a.device_id WHERE a.enabled=1 AND a.state='ready' AND a.number<>''")
	if err != nil {
		return err
	}
	assignments := map[string]string{}
	for rows.Next() {
		var u, n string
		if err = rows.Scan(&u, &n); err != nil {
			rows.Close()
			return err
		}
		assignments[u] = n
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	raw, _ := json.Marshal(assignments)
	if _, err = tx.ExecContext(ctx, "INSERT INTO config(key,value) VALUES('pbx_device_numbers',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", string(raw)); err != nil {
		return err
	}
	return tx.Commit()
}
func (h *BusinessHandler) Audit(w http.ResponseWriter, r *http.Request) {
	rows, err := h.deps.DB.Conn().QueryContext(r.Context(), "SELECT id,actor_id,action,subject,outcome,created_at FROM business_audit ORDER BY id DESC LIMIT 100")
	if err != nil {
		WriteInternalError(w)
		return
	}
	defer rows.Close()
	result := []map[string]interface{}{}
	for rows.Next() {
		var id int64
		var actor sql.NullInt64
		var a, s, o, t string
		if rows.Scan(&id, &actor, &a, &s, &o, &t) != nil {
			WriteInternalError(w)
			return
		}
		result = append(result, map[string]interface{}{"id": id, "actor_id": actor.Int64, "action": a, "subject": s, "outcome": o, "created_at": t})
	}
	WriteJSON(w, 200, map[string]interface{}{"data": result})
}

func (h *BusinessHandler) Disable(w http.ResponseWriter, r *http.Request) {
	http.NewResponseController(w).SetWriteDeadline(time.Now().Add(120 * time.Second))
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	businessMu.Lock()
	defer businessMu.Unlock()
	if err := h.bootstrap(r.Context()); err != nil {
		WriteInternalError(w)
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteValidationError(w, "Invalid SIP user", nil)
		return
	}
	device, err := h.deps.DB.Devices.GetByID(r.Context(), id)
	if err != nil {
		WriteNotFoundError(w, "SIP user")
		return
	}
	var linkedNumber, linkedState string
	if err = h.deps.DB.Conn().QueryRowContext(r.Context(), "SELECT number,state FROM sip_accounts WHERE device_id=?", id).Scan(&linkedNumber, &linkedState); err != nil {
		WriteInternalError(w)
		return
	}
	if linkedState == "deleted" {
		WriteNotFoundError(w, "SIP user")
		return
	}
	if err = checkNumberProtection(r.Context(), h.deps.DB, linkedNumber); err != nil {
		WriteError(w, 409, "NUMBER_PROTECTED", err.Error(), nil)
		return
	}
	// Disable locally first, even if Twilio is unavailable. Retry revokes any
	// remaining Twilio credential; history and the owned number are preserved.
	tx, err := h.deps.DB.Conn().BeginTx(r.Context(), nil)
	if err != nil {
		WriteInternalError(w)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), "UPDATE sip_accounts SET enabled=0,state='disabled',updated_at=CURRENT_TIMESTAMP WHERE device_id=?", id); err != nil {
		WriteInternalError(w)
		return
	}
	var raw string
	if err = tx.QueryRowContext(r.Context(), "SELECT value FROM config WHERE key='pbx_device_numbers'").Scan(&raw); err == sql.ErrNoRows {
		raw = "{}"
	} else if err != nil {
		WriteInternalError(w)
		return
	}
	var assignments map[string]string
	if json.Unmarshal([]byte(raw), &assignments) != nil {
		WriteInternalError(w)
		return
	}
	if assignments == nil {
		assignments = map[string]string{}
	}
	delete(assignments, device.Username)
	body, _ := json.Marshal(assignments)
	if _, err = tx.ExecContext(r.Context(), "INSERT INTO config(key,value) VALUES('pbx_device_numbers',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", string(body)); err != nil {
		WriteInternalError(w)
		return
	}
	if _, err = tx.ExecContext(r.Context(), "UPDATE routes SET enabled=0 WHERE did_id=(SELECT id FROM dids WHERE number=(SELECT number FROM sip_accounts WHERE device_id=?))", id); err != nil {
		WriteInternalError(w)
		return
	}
	if err = tx.Commit(); err != nil {
		WriteInternalError(w)
		return
	}
	listSID, err := h.credentialList(r.Context())
	if err == nil {
		var credential string
		credential, err = h.findCredential(r.Context(), listSID, device.Username)
		if err == nil && credential != "" {
			err = h.twilio(r.Context(), "DELETE", "/SIP/CredentialLists/"+listSID+"/Credentials/"+credential+".json", nil, nil)
		}
	}
	if err != nil {
		h.audit(r.Context(), getUserIDFromContext(r.Context()), "disable", device.Username, "local_only")
		WriteError(w, 502, "REVOCATION_PENDING", "SIP user is disabled locally. Twilio credential revocation failed; retry Disable", nil)
		return
	}
	h.audit(r.Context(), getUserIDFromContext(r.Context()), "disable", device.Username, "disabled")
	WriteJSON(w, 200, map[string]string{"state": "disabled"})
}
func (h *BusinessHandler) Status(w http.ResponseWriter, r *http.Request) {
	req, _ := http.NewRequestWithContext(r.Context(), "GET", h.deps.Config.PBXURL+"/health", nil)
	healthy := false
	if req != nil {
		res, err := h.client.Do(req)
		if err == nil {
			healthy = res.StatusCode == 200
			res.Body.Close()
		}
	}
	backup := map[string]interface{}{}
	if req, err := http.NewRequestWithContext(r.Context(), "GET", h.deps.Config.PBXURL+"/backup-status", nil); err == nil {
		req.Header.Set("Authorization", "Bearer "+h.deps.Config.PBXSecret)
		if res, err := h.client.Do(req); err == nil {
			if res.StatusCode == 200 {
				json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&backup)
			}
			res.Body.Close()
		}
	}
	sid, token := h.deps.Config.TwilioCredentials()
	WriteJSON(w, 200, map[string]interface{}{"pbx_healthy": healthy, "twilio_configured": sid != "" && token != "", "last_backup": backup["last_success"], "sip_server": h.deps.Config.PBXSIPDomain, "transport": "TLS 5061 / SRTP", "console_access": "Administrators only", "number_source": "Existing Twilio numbers", "credential_storage": "Twilio API credentials in environment; phone password hashes in database; SIP trunk secrets in private PBX volume"})
}
