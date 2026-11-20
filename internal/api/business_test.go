package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/rohitkumar-co-in/leadomi-sip/internal/config"
	"github.com/rohitkumar-co-in/leadomi-sip/internal/models"
	"github.com/rohitkumar-co-in/leadomi-sip/pkg/sip"
)

type businessTransport func(*http.Request) (*http.Response, error)

func (f businessTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestBusinessProvisioningLifecycle(t *testing.T) {
	setup := setupTestAPI(t)
	ctx := context.Background()
	suffix := strings.Repeat("a", 32)
	sid := "AC" + suffix
	list := "CL" + suffix
	domain := "SD" + suffix
	pn := "PN" + suffix
	cred := "CR" + suffix
	cfg := &config.Config{TwilioAccountSID: sid, TwilioAuthToken: "secret-token", TwilioSIPDomain: "test.sip.twilio.com", PublicURL: "https://sip.example.test", PBXSIPDomain: "sip.example.test", PBXURL: "http://pbx.test", PBXSecret: "pbx-secret"}
	deps := &Dependencies{DB: setup.DB, Config: cfg}
	h := newBusinessHandler(deps)
	setup.DB.Config.Set(ctx, "pbx_twilio_domain_sid", domain)
	setup.DB.Config.Set(ctx, "pbx_twilio_credential_list_sid", list)
	credentialExists := false
	credentialPassword := ""
	pbxPassword := ""
	numberUpdated := false
	failPBX := false
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/credentials") {
			if failPBX {
				w.WriteHeader(503)
				return
			}
			if r.Header.Get("Authorization") != "Bearer pbx-secret" {
				t.Error("Missing private PBX authentication")
			}
			var v map[string]string
			json.NewDecoder(r.Body).Decode(&v)
			pbxPassword = v["password"]
			fmt.Fprint(w, `{"saved":true}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/registrations") {
			fmt.Fprint(w, `[]`)
			return
		}
		user, token, _ := r.BasicAuth()
		if user != sid || token != "secret-token" {
			t.Error("Missing Twilio authentication")
		}
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/IncomingPhoneNumbers.json"):
			fmt.Fprintf(w, `{"incoming_phone_numbers":[{"sid":%q,"phone_number":"+442345678901","capabilities":{"voice":true,"sms":true}}]}`, pn)
		case strings.HasSuffix(path, "/Domains/"+domain+".json"):
			fmt.Fprint(w, `{"domain_name":"test.sip.twilio.com","secure":true}`)
		case strings.HasSuffix(path, "/CredentialListMappings.json"):
			fmt.Fprintf(w, `{"contents":[{"sid":%q}]}`, list)
		case strings.HasSuffix(path, "/Credentials.json") && r.Method == "GET":
			if credentialExists {
				fmt.Fprintf(w, `{"credentials":[{"sid":%q,"username":"employee"}]}`, cred)
			} else {
				fmt.Fprint(w, `{"credentials":[]}`)
			}
		case strings.Contains(path, "/Credentials") && r.Method == "POST":
			r.ParseForm()
			credentialPassword = r.Form.Get("Password")
			credentialExists = true
			fmt.Fprintf(w, `{"sid":%q}`, cred)
		case strings.Contains(path, "/Credentials/") && r.Method == "DELETE":
			credentialExists = false
			w.WriteHeader(204)
		case strings.HasSuffix(path, "/Services"):
			fmt.Fprint(w, `{"services":[],"meta":{}}`)
		case strings.HasSuffix(path, "/IncomingPhoneNumbers/"+pn+".json"):
			r.ParseForm()
			if r.Form.Get("SmsUrl") != cfg.PublicURL+"/api/webhooks/sms/incoming" || r.Form.Get("VoiceUrl") != cfg.PublicURL+"/api/webhooks/voice/incoming" {
				t.Error("Number callbacks not configured")
			}
			numberUpdated = true
			fmt.Fprint(w, `{}`)
		default:
			t.Errorf("Unexpected fake request: %s %s", r.Method, path)
			w.WriteHeader(404)
		}
	}))
	defer fake.Close()
	fakeURL, _ := url.Parse(fake.URL)
	h.client = &http.Client{Transport: businessTransport(func(r *http.Request) (*http.Response, error) {
		copy := r.Clone(r.Context())
		u := *r.URL
		u.Scheme = fakeURL.Scheme
		u.Host = fakeURL.Host
		copy.URL = &u
		return http.DefaultTransport.RoundTrip(copy)
	})}
	invoke := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), contextKeyUser, &models.User{ID: 1, Role: "admin"}))
		rec := httptest.NewRecorder()
		router := chi.NewRouter()
		router.Post("/accounts", h.Provision)
		router.Put("/accounts/{id}", h.Provision)
		router.Post("/accounts/{id}/disable", h.Disable)
		router.Delete("/accounts/{id}", h.Delete)
		router.ServeHTTP(rec, req)
		return rec
	}
	blocked := invoke("POST", "/accounts", `{"name":"Employee","username":"employee","number":"+442345678901"}`)
	assertStatus(t, blocked, 409)
	if credentialExists || numberUpdated {
		t.Fatal("Unreviewed assignment wrote provider configuration")
	}
	var count int
	setup.DB.Conn().QueryRow("SELECT COUNT(*) FROM devices WHERE username='employee'").Scan(&count)
	if count != 0 {
		t.Fatal("Unreviewed assignment created a local user")
	}
	numbers, _ := h.numbers(ctx)
	review, reviewErr := h.reviewNumber(ctx, numbers[0])
	if reviewErr != nil {
		t.Fatal(reviewErr)
	}
	result := invoke("POST", "/accounts", fmt.Sprintf(`{"name":"Employee","username":"employee","number":"+442345678901","number_review":%q}`, review.Fingerprint))
	assertStatus(t, result, 200)
	var response struct {
		ID       int64  `json:"id"`
		Password string `json:"password"`
	}
	json.Unmarshal(result.Body.Bytes(), &response)
	if len(response.Password) < 24 || credentialPassword == response.Password || pbxPassword != credentialPassword || !numberUpdated {
		t.Fatal("Separate phone/PBX credential provisioning failed")
	}
	device, _ := setup.DB.Devices.GetByID(ctx, response.ID)
	if device.PasswordHash != sip.GenerateHA1("employee", "gosip", response.Password) {
		t.Fatal("Phone password was not hashed")
	}
	did, _ := setup.DB.DIDs.GetByNumber(ctx, "+442345678901")
	if did.TwilioSID != pn {
		t.Fatal("Number SID was lost")
	}
	routes, _ := setup.DB.Routes.GetByDID(ctx, did.ID)
	if len(routes) != 1 || !strings.Contains(string(routes[0].ActionData), `"devices":[`) {
		t.Fatal("Incoming call/SMS route is incompatible")
	}
	var audit int
	setup.DB.Conn().QueryRow("SELECT COUNT(*) FROM business_audit").Scan(&audit)
	if audit != 1 {
		t.Fatal("No provisioning audit entry")
	}
	conflict := invoke("POST", "/accounts", `{"name":"Other","username":"other","number":"+442345678901"}`)
	assertStatus(t, conflict, 409)
	owned := invoke("POST", "/accounts", `{"name":"Other","username":"other","number":"+442345678902"}`)
	assertStatus(t, owned, 400)
	// Ordinary edits and phone password changes must not write Twilio or PBX credentials.
	failPBX = true
	previousCredential := credentialPassword
	numberUpdated = false
	edited := invoke("PUT", fmt.Sprintf("/accounts/%d", response.ID), `{"name":"Renamed","number":"+442345678901"}`)
	assertStatus(t, edited, 200)
	if numberUpdated || credentialPassword != previousCredential {
		t.Fatal("Ordinary edit changed an existing provider connection")
	}
	custom := "CustomPhonePassword123!"
	changed := invoke("PUT", fmt.Sprintf("/accounts/%d", response.ID), fmt.Sprintf(`{"name":"Renamed","number":"+442345678901","reset_password":true,"password":%q}`, custom))
	assertStatus(t, changed, 200)
	device, _ = setup.DB.Devices.GetByID(ctx, response.ID)
	if device.PasswordHash != sip.GenerateHA1("employee", "gosip", custom) {
		t.Fatal("Custom password was not hashed")
	}
	if numberUpdated || credentialPassword != previousCredential {
		t.Fatal("Phone password reset changed Twilio credentials")
	}
	assertStatus(t, invoke("DELETE", fmt.Sprintf("/accounts/%d", response.ID), ""), 409)
	if err := setup.DB.Config.Set(ctx, "business_excluded_numbers", `[{"number":"+442345678901","reason":"Other service"}]`); err != nil {
		t.Fatal(err)
	}
	protectedReview, err := h.reviewNumber(ctx, numbers[0])
	if err != nil || !protectedReview.Blocked {
		t.Fatal("Excluded review was not blocked")
	}
	assertStatus(t, invoke("PUT", fmt.Sprintf("/accounts/%d", response.ID), `{"name":"Protected","number":"+442345678901"}`), 409)
	assertStatus(t, invoke("POST", fmt.Sprintf("/accounts/%d/disable", response.ID), ""), 409)
	assertStatus(t, invoke("DELETE", fmt.Sprintf("/accounts/%d", response.ID), ""), 409)
	if !credentialExists {
		t.Fatal("Protected account credential changed")
	}
	setup.DB.Config.Set(ctx, "business_excluded_numbers", "[]")
	failPBX = false
	disabled := invoke("POST", fmt.Sprintf("/accounts/%d/disable", response.ID), "")
	assertStatus(t, disabled, 200)
	if credentialExists {
		t.Fatal("Disabled user's Twilio credential was not revoked")
	}
	if _, err := deviceOutboundNumber(ctx, deps, "employee"); err == nil {
		t.Fatal("Disabled user retained outgoing number")
	}
	routes, _ = setup.DB.Routes.GetByDID(ctx, did.ID)
	if len(routes) != 1 || routes[0].Enabled {
		t.Fatal("Disabled user still receives inbound calls/SMS")
	}
	// Deletion must fail closed while provider revocation cannot be verified.
	workingClient := h.client
	h.client = &http.Client{Transport: businessTransport(func(*http.Request) (*http.Response, error) { return nil, fmt.Errorf("offline") })}
	assertStatus(t, invoke("DELETE", fmt.Sprintf("/accounts/%d", response.ID), ""), 502)
	h.client = workingClient
	assertStatus(t, invoke("DELETE", fmt.Sprintf("/accounts/%d", response.ID), ""), 200)
	assertStatus(t, invoke("DELETE", fmt.Sprintf("/accounts/%d", response.ID), ""), 200)
	var state, number, hash string
	setup.DB.Conn().QueryRow("SELECT state,number FROM sip_accounts WHERE device_id=?", response.ID).Scan(&state, &number)
	setup.DB.Conn().QueryRow("SELECT password_hash FROM devices WHERE id=?", response.ID).Scan(&hash)
	if state != "deleted" || number != "" || hash != "" {
		t.Fatal("Deletion left usable phone access or assignment")
	}
	if _, err = setup.DB.DIDs.GetByID(ctx, did.ID); err != nil {
		t.Fatal("Deletion removed owned number")
	}
	assertStatus(t, invoke("PUT", fmt.Sprintf("/accounts/%d", response.ID), `{"name":"Resurrect","number":"+442345678901"}`), 404)
	// A failed first provision may never have created an assignment map.
	pending := &models.Device{Name: "Incomplete", Username: "incomplete", PasswordHash: sip.GenerateHA1("incomplete", "gosip", "synthetic-password"), DeviceType: "linphone"}
	if err = setup.DB.Devices.Create(ctx, pending); err != nil {
		t.Fatal(err)
	}
	setup.DB.Conn().Exec("INSERT INTO sip_accounts(device_id,state) VALUES(?,'error')", pending.ID)
	setup.DB.Conn().Exec("DELETE FROM config WHERE key='pbx_device_numbers'")
	assertStatus(t, invoke("POST", fmt.Sprintf("/accounts/%d/disable", pending.ID), ""), 200)
	assertStatus(t, invoke("DELETE", fmt.Sprintf("/accounts/%d", pending.ID), ""), 200)

}
func TestBusinessConsoleBoundary(t *testing.T) {
	for _, username := range []string{"twilio-in", "twilio-out-3", "global", "tls", "Twilio-out"} {
		if validBusinessUsername(username) {
			t.Fatalf("Internal endpoint name allowed: %s", username)
		}
	}
	if !validBusinessUsername("employee_3") {
		t.Fatal("Valid employee username rejected")
	}
	deps := &Dependencies{Config: &config.Config{PBXURL: "http://pbx", PublicURL: "https://sip.test"}}
	for _, test := range []struct {
		role, method, path, origin string
		want                       int
	}{{"user", "GET", "/api/messages", "", 403}, {"admin", "POST", "/api/devices", "", 409}, {"admin", "POST", "/api/business/accounts", "https://evil.test", 403}, {"admin", "GET", "/api/messages", "", 204}} {
		req := httptest.NewRequest(test.method, test.path, nil)
		req.Header.Set("Origin", test.origin)
		req = req.WithContext(context.WithValue(req.Context(), contextKeyUser, &models.User{Role: test.role}))
		rec := httptest.NewRecorder()
		BusinessConsoleBoundary(deps)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })).ServeHTTP(rec, req)
		assertStatus(t, rec, test.want)
	}
}
func TestBusinessPolicyAndSessionRevocation(t *testing.T) {
	setup := setupTestAPI(t)
	ctx := context.Background()
	deps := &Dependencies{DB: setup.DB, Config: &config.Config{PBXURL: "http://pbx"}}
	setup.DB.Config.Set(ctx, "business_policy", `{"allowed_prefixes":["+44","+91"],"sms_per_minute":1}`)
	if !businessDestinationAllowed(ctx, deps, "+441234567890") || businessDestinationAllowed(ctx, deps, "+11234567890") {
		t.Fatal("Country restrictions were not enforced")
	}
	if !businessSMSLimit(ctx, deps, "+441234567890") {
		t.Fatal("First SMS should be allowed")
	}
	setup.DB.Conn().Exec("INSERT INTO messages(direction,from_number,to_number,body,status,created_at) VALUES('outbound','+441234567890','+441234567891','test','queued',CURRENT_TIMESTAMP)")
	if businessSMSLimit(ctx, deps, "+441234567890") {
		t.Fatal("SMS rate limit was not enforced")
	}
	setup.DB.Config.Set(ctx, "business_policy", `invalid`)
	if businessDestinationAllowed(ctx, deps, "+441234567890") {
		t.Fatal("Corrupt usage policy must fail closed")
	}
	user := createTestUser(t, setup.DB, "admin@company.test", "long-test-password", "admin")
	token, err := createSessionWithRequest(ctx, setup.DB, user.ID, "test", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = validateSession(ctx, setup.DB, token); err != nil {
		t.Fatal(err)
	}
	setup.DB.Sessions.DeleteByUserID(ctx, user.ID)
	if _, err = validateSession(ctx, setup.DB, token); err == nil {
		t.Fatal("Revoked cached session was accepted")
	}
}

type bootstrapFake struct {
	h                *BusinessHandler
	cfg              *config.Config
	setup            *testSetup
	rotated, pbxPass string
	deleted, created bool
	writes           int
}

// newBootstrapFake builds a business handler whose Twilio account already has a
// "leadomi" bootstrap credential and owns +447782226484.
func newBootstrapFake(t *testing.T) *bootstrapFake {
	setup := setupTestAPI(t)
	suffix := strings.Repeat("b", 32)
	sid, list, domain, pn, cred := "AC"+suffix, "CL"+suffix, "SD"+suffix, "PN"+suffix, "CR"+suffix
	f := &bootstrapFake{setup: setup}
	f.cfg = &config.Config{TwilioAccountSID: sid, TwilioAuthToken: "secret-token", TwilioSIPDomain: "test.sip.twilio.com",
		TwilioSIPDomainSID: domain, TwilioCredentialListSID: list, TwilioBootstrapUser: "leadomi",
		PublicURL: "https://sip.example.test", PBXSIPDomain: "sip.example.test", PBXURL: "http://pbx.test", PBXSecret: "pbx-secret"}
	f.h = newBusinessHandler(&Dependencies{DB: setup.DB, Config: f.cfg})
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if r.Method != "GET" {
			f.writes++
		}
		switch {
		case strings.HasSuffix(path, "/credentials"):
			var v map[string]string
			json.NewDecoder(r.Body).Decode(&v)
			if v["username"] != "leadomi" {
				t.Error("PBX credential saved for wrong user")
			}
			f.pbxPass = v["password"]
			fmt.Fprint(w, `{"saved":true}`)
		case strings.HasSuffix(path, "/registrations"):
			fmt.Fprint(w, `[]`)
		case strings.HasSuffix(path, "/IncomingPhoneNumbers.json"):
			fmt.Fprintf(w, `{"incoming_phone_numbers":[{"sid":%q,"phone_number":"+447782226484","capabilities":{"voice":true,"sms":true}}]}`, pn)
		case strings.HasSuffix(path, "/Domains/"+domain+".json"):
			fmt.Fprint(w, `{"domain_name":"test.sip.twilio.com","secure":true}`)
		case strings.HasSuffix(path, "/CredentialListMappings.json"):
			fmt.Fprintf(w, `{"contents":[{"sid":%q}]}`, list)
		case strings.HasSuffix(path, "/Credentials.json") && r.Method == "GET":
			fmt.Fprintf(w, `{"credentials":[{"sid":%q,"username":"leadomi"}]}`, cred)
		case strings.HasSuffix(path, "/Credentials/"+cred+".json") && r.Method == "POST":
			r.ParseForm()
			f.rotated = r.Form.Get("Password")
			fmt.Fprintf(w, `{"sid":%q}`, cred)
		case strings.HasSuffix(path, "/Credentials.json") && r.Method == "POST":
			f.created = true
			fmt.Fprintf(w, `{"sid":%q}`, cred)
		case strings.Contains(path, "/Credentials/") && r.Method == "DELETE":
			f.deleted = true
			w.WriteHeader(204)
		case strings.HasSuffix(path, "/Services"):
			fmt.Fprint(w, `{"services":[],"meta":{}}`)
		case strings.HasSuffix(path, "/IncomingPhoneNumbers/"+pn+".json"):
			fmt.Fprint(w, `{}`)
		default:
			t.Errorf("Unexpected fake request: %s %s", r.Method, path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(fake.Close)
	fakeURL, _ := url.Parse(fake.URL)
	f.h.client = &http.Client{Transport: businessTransport(func(r *http.Request) (*http.Response, error) {
		copy := r.Clone(r.Context())
		u := *r.URL
		u.Scheme, u.Host = fakeURL.Scheme, fakeURL.Host
		copy.URL = &u
		return http.DefaultTransport.RoundTrip(copy)
	})}
	return f
}

// A phone user may share the server's bootstrap Twilio credential username.
// Provisioning adopts and rotates that credential; disabling never deletes it.
func TestBusinessBootstrapUsernameProvisioning(t *testing.T) {
	f := newBootstrapFake(t)
	ctx := context.Background()
	invoke := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), contextKeyUser, &models.User{ID: 1, Role: "admin"}))
		rec := httptest.NewRecorder()
		router := chi.NewRouter()
		router.Post("/accounts", f.h.Provision)
		router.Post("/accounts/{id}/disable", f.h.Disable)
		router.ServeHTTP(rec, req)
		return rec
	}
	numbers, err := f.h.numbers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	review, err := f.h.reviewNumber(ctx, numbers[0])
	if err != nil {
		t.Fatal(err)
	}
	result := invoke("POST", "/accounts", fmt.Sprintf(`{"name":"Leadomi","username":"leadomi","number":"+447782226484","number_review":%q}`, review.Fingerprint))
	assertStatus(t, result, 200)
	if f.created || f.rotated == "" || f.pbxPass != f.rotated {
		t.Fatal("Bootstrap credential was not adopted and rotated into the PBX")
	}
	var response struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(result.Body.Bytes(), &response)
	assertStatus(t, invoke("POST", fmt.Sprintf("/accounts/%d/disable", response.ID), ""), 200)
	if f.deleted {
		t.Fatal("Disabling the phone user deleted the server's bootstrap credential")
	}
}

// The environment-defined phone user is provisioned once; restarts do not write
// to Twilio, and a changed GOSIP_PHONE_PASSWORD only updates the local digest.
func TestEnvPhoneUserSync(t *testing.T) {
	f := newBootstrapFake(t)
	ctx := context.Background()
	f.cfg.OutboundCallerID = "+447782226484"
	f.cfg.PhonePassword = "FirstPhonePassword1"
	if err := f.h.EnsureEnvPhoneUser(ctx); err != nil {
		t.Fatal(err)
	}
	device, err := f.setup.DB.Devices.GetByUsername(ctx, "leadomi")
	if err != nil || device.PasswordHash != sip.GenerateHA1("leadomi", "gosip", "FirstPhonePassword1") {
		t.Fatal("Environment phone user was not created with its password")
	}
	var number, state string
	f.setup.DB.Conn().QueryRow("SELECT number,state FROM sip_accounts WHERE device_id=?", device.ID).Scan(&number, &state)
	if number != "+447782226484" || state != "ready" || f.pbxPass == "" {
		t.Fatal("Environment phone user was not provisioned")
	}
	writes := f.writes
	if err = f.h.EnsureEnvPhoneUser(ctx); err != nil || f.writes != writes {
		t.Fatal("A restart re-provisioned a ready phone user")
	}
	f.cfg.PhonePassword = "SecondPhonePassword2"
	if err = f.h.EnsureEnvPhoneUser(ctx); err != nil || f.writes != writes {
		t.Fatal("A phone password change wrote to Twilio")
	}
	device, _ = f.setup.DB.Devices.GetByUsername(ctx, "leadomi")
	if device.PasswordHash != sip.GenerateHA1("leadomi", "gosip", "SecondPhonePassword2") {
		t.Fatal("Phone password change was not applied")
	}
	// Choosing the number in the console requires the current review.
	selectNumber := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("PUT", "/phone-number", strings.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), contextKeyUser, &models.User{ID: 1, Role: "admin"}))
		rec := httptest.NewRecorder()
		f.h.SelectPhoneNumber(rec, req)
		return rec
	}
	assertStatus(t, selectNumber(`{"number":"+447782226484","number_review":"stale"}`), 409)
	numbers, _ := f.h.numbers(ctx)
	review, _ := f.h.reviewNumber(ctx, numbers[0])
	assertStatus(t, selectNumber(fmt.Sprintf(`{"number":"+447782226484","number_review":%q}`, review.Fingerprint)), 200)
	if f.h.phoneNumber(ctx) != "+447782226484" {
		t.Fatal("Chosen phone number was not stored")
	}
	assertStatus(t, selectNumber(`{"number":"+15550000000","number_review":"x"}`), 400)
	f.cfg.PhonePassword = "short"
	if f.h.EnsureEnvPhoneUser(ctx) == nil {
		t.Fatal("Invalid phone password was accepted")
	}
}
