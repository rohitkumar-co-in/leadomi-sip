package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// setupCheck is one line of the dashboard's setup checklist.
type setupCheck struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Status string `json:"status"` // ok | fail | warn | skip
	Detail string `json:"detail,omitempty"`
	Fix    string `json:"fix,omitempty"`
}

// SetupCheck verifies the Twilio account, SIP domain, number and phone
// against this server's configuration. It only reads from Twilio.
func (h *BusinessHandler) SetupCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	checks := h.runSetupChecks(ctx)
	ok := 0
	for _, c := range checks {
		if c.Status == "ok" {
			ok++
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	WriteJSON(w, http.StatusOK, map[string]interface{}{"checks": checks, "ok": ok, "total": len(checks)})
}

func (h *BusinessHandler) runSetupChecks(ctx context.Context) []setupCheck {
	cfg := h.deps.Config
	base := strings.TrimRight(cfg.PublicURL, "/")
	var checks []setupCheck
	add := func(id, label, status, detail, fix string) {
		checks = append(checks, setupCheck{ID: id, Label: label, Status: status, Detail: detail, Fix: fix})
	}
	skip := func(id, label, why string) { add(id, label, "skip", why, "") }

	// 1. Account credentials.
	var account struct {
		Status string `json:"status"`
		Name   string `json:"friendly_name"`
	}
	accountOK := false
	if sid, token := cfg.TwilioCredentials(); sid == "" || token == "" {
		add("account", "Twilio account keys", "fail", "TWILIO_ACCOUNT_SID or TWILIO_AUTH_TOKEN is empty.", "Set both in Coolify and redeploy.")
	} else if err := h.twilio(ctx, http.MethodGet, ".json", nil, &account); err != nil {
		add("account", "Twilio account keys", "fail", "Twilio rejected the keys ("+err.Error()+").", "Copy the Account SID and Auth Token again from the Twilio console home page.")
	} else if account.Status != "active" {
		add("account", "Twilio account keys", "fail", "Account status is "+account.Status+".", "Reactivate or upgrade the account in Twilio.")
	} else {
		accountOK = true
		add("account", "Twilio account keys", "ok", "Connected to "+account.Name+".", "")
	}

	// 2-4. SIP domain.
	domainSID, err := h.deps.DB.Config.Get(ctx, "pbx_twilio_domain_sid")
	if err != nil || !resourceSID.MatchString(domainSID) {
		domainSID = cfg.TwilioSIPDomainSID
	}
	var domain struct {
		Name         string `json:"domain_name"`
		Secure       bool   `json:"secure"`
		Registration bool   `json:"sip_registration"`
		VoiceURL     string `json:"voice_url"`
		VoiceMethod  string `json:"voice_method"`
		FallbackURL  string `json:"voice_fallback_url"`
	}
	domainOK := false
	switch {
	case !accountOK:
		skip("sip_domain", "SIP domain", "Needs working account keys.")
	case !resourceSID.MatchString(domainSID):
		add("sip_domain", "SIP domain", "fail", "TWILIO_SIP_DOMAIN_SID is missing or invalid.", "Copy the SIP domain SID (SD…) from Twilio → Voice → SIP domains into Coolify.")
	default:
		if err := h.twilio(ctx, http.MethodGet, "/SIP/Domains/"+domainSID+".json", nil, &domain); err != nil {
			add("sip_domain", "SIP domain", "fail", "Twilio could not find SIP domain "+domainSID+".", "Check TWILIO_SIP_DOMAIN_SID belongs to this Twilio account.")
		} else if domain.Name != cfg.TwilioSIPDomain {
			add("sip_domain", "SIP domain", "fail", fmt.Sprintf("Twilio domain is %s but TWILIO_SIP_DOMAIN is %q.", domain.Name, cfg.TwilioSIPDomain), "Make TWILIO_SIP_DOMAIN exactly match the SIP URI in Twilio.")
		} else {
			domainOK = true
			add("sip_domain", "SIP domain", "ok", domain.Name, "")
		}
	}
	if domainOK {
		if domain.Secure && domain.Registration {
			add("domain_security", "Secure media and SIP registration", "ok", "Both enabled.", "")
		} else {
			var missing []string
			if !domain.Secure {
				missing = append(missing, "Secure media")
			}
			if !domain.Registration {
				missing = append(missing, "SIP registration")
			}
			add("domain_security", "Secure media and SIP registration", "fail", strings.Join(missing, " and ")+" disabled.", "Enable them on the SIP domain page in Twilio.")
		}
		wantVoice := base + "/api/webhooks/voice/outgoing"
		switch {
		case domain.VoiceURL != wantVoice || !strings.EqualFold(domain.VoiceMethod, "POST"):
			add("domain_voice_url", "Outgoing calls webhook", "fail", fmt.Sprintf("“A call comes in” is %q (%s).", domain.VoiceURL, domain.VoiceMethod), "Set it to "+wantVoice+" with HTTP POST.")
		case domain.FallbackURL == "":
			add("domain_voice_url", "Outgoing calls webhook", "warn", "Set correctly, but no “Primary handler fails” fallback.", "Add a TwiML Bin fallback so callers hear a message if the server is down.")
		default:
			add("domain_voice_url", "Outgoing calls webhook", "ok", wantVoice, "")
		}
	} else {
		skip("domain_security", "Secure media and SIP registration", "Needs the SIP domain.")
		skip("domain_voice_url", "Outgoing calls webhook", "Needs the SIP domain.")
	}

	// 5-6. Credential list and bootstrap credential.
	listSID, err := h.deps.DB.Config.Get(ctx, "pbx_twilio_credential_list_sid")
	if err != nil || !resourceSID.MatchString(listSID) {
		listSID = cfg.TwilioCredentialListSID
	}
	listOK := false
	switch {
	case !domainOK:
		skip("credential_list", "Credential list attached", "Needs the SIP domain.")
	case !resourceSID.MatchString(listSID):
		add("credential_list", "Credential list attached", "fail", "TWILIO_CREDENTIAL_LIST_SID is missing or invalid.", "Copy the credential list SID (CL…) from Twilio into Coolify.")
	default:
		var missing []string
		for _, kind := range []string{"Calls", "Registrations"} {
			var mappings struct {
				Items []struct {
					SID string `json:"sid"`
				} `json:"contents"`
			}
			found := false
			if err := h.twilio(ctx, http.MethodGet, "/SIP/Domains/"+domainSID+"/Auth/"+kind+"/CredentialListMappings.json?PageSize=1000", nil, &mappings); err == nil {
				for _, item := range mappings.Items {
					found = found || item.SID == listSID
				}
			}
			if !found {
				missing = append(missing, kind)
			}
		}
		if len(missing) > 0 {
			add("credential_list", "Credential list attached", "fail", "Not attached for "+strings.Join(missing, " and ")+".", "On the SIP domain, select the credential list under Voice authentication and SIP registration.")
		} else {
			listOK = true
			add("credential_list", "Credential list attached", "ok", "Attached for calls and registrations.", "")
		}
	}
	switch {
	case !listOK:
		skip("credential", "SIP login exists", "Needs the credential list.")
	case cfg.TwilioBootstrapUser == "":
		add("credential", "SIP login exists", "fail", "PBX_TWILIO_USER is empty.", "Set it to the username you created in the credential list.")
	default:
		if credential, err := h.findCredential(ctx, listSID, cfg.TwilioBootstrapUser); err != nil || credential == "" {
			add("credential", "SIP login exists", "fail", "No credential named "+cfg.TwilioBootstrapUser+" in the list.", "Add it in Twilio, or set PBX_TWILIO_USER to the existing username.")
		} else {
			add("credential", "SIP login exists", "ok", cfg.TwilioBootstrapUser, "")
		}
	}

	// 7-8. Phone number.
	number := phoneNumberFor(ctx, h.deps)
	var owned *businessNumber
	numberOK := false
	switch {
	case !accountOK:
		skip("number", "Phone number in this account", "Needs working account keys.")
	case !e164Destination.MatchString(number):
		add("number", "Phone number in this account", "fail", "No number chosen.", "Set GOSIP_OUTBOUND_CALLER_ID, or choose one under Phone Numbers.")
	default:
		if numbers, err := h.numbers(ctx); err == nil {
			for i := range numbers {
				if numbers[i].Number == number {
					owned = &numbers[i]
				}
			}
		}
		switch {
		case owned == nil:
			add("number", "Phone number in this account", "fail", number+" is not in this Twilio account.", "Buy or move the number into this account, or choose another under Phone Numbers.")
		case !owned.Capabilities.Voice || !owned.Capabilities.SMS:
			add("number", "Phone number in this account", "fail", number+" lacks voice or SMS.", "Use a number with both Voice and SMS.")
		default:
			numberOK = true
			add("number", "Phone number in this account", "ok", number, "")
		}
	}
	if numberOK {
		want := map[string]string{"Calls": base + "/api/webhooks/voice/incoming", "SMS": base + "/api/webhooks/sms/incoming", "Call status": base + "/api/webhooks/voice/status"}
		got := map[string]string{"Calls": owned.VoiceURL, "SMS": owned.SMSURL, "Call status": owned.StatusURL}
		var wrong []string
		for _, name := range []string{"Calls", "SMS", "Call status"} {
			if got[name] != want[name] {
				wrong = append(wrong, name)
			}
		}
		if len(wrong) == 0 {
			add("number_webhooks", "Number webhooks point here", "ok", "Calls, SMS and call status reach this server.", "")
		} else {
			add("number_webhooks", "Number webhooks point here", "fail", strings.Join(wrong, ", ")+" webhook(s) point elsewhere.", "They are set automatically when the phone user is provisioned; otherwise set them in Twilio to "+base+"/api/webhooks/….")
		}
	} else {
		skip("number_webhooks", "Number webhooks point here", "Needs the phone number.")
	}

	// 9. Server side: PBX, phone user, registration.
	healthy := false
	if req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.PBXURL+"/health", nil); err == nil {
		if res, err := h.client.Do(req); err == nil {
			healthy = res.StatusCode == http.StatusOK
			res.Body.Close()
		}
	}
	if healthy {
		add("pbx", "Phone server running", "ok", "Asterisk is running with a valid certificate.", "")
	} else {
		add("pbx", "Phone server running", "fail", "The phone server is not healthy.", "Usually missing Twilio values or no TLS certificate yet; check the pbx container logs in Coolify.")
	}
	// Call audio goes to PBX_PUBLIC_IP. If the VPS IP changes (e.g. an AWS
	// instance restarted without an Elastic IP), calls ring but have no sound.
	if domain := phoneServer(cfg.PBXSIPDomain); domain != "" {
		addrs, err := net.DefaultResolver.LookupHost(ctx, domain)
		var v4 []string
		for _, a := range addrs {
			if ip := net.ParseIP(a); ip != nil && ip.To4() != nil {
				v4 = append(v4, a)
			}
		}
		matched := false
		for _, a := range v4 {
			matched = matched || a == cfg.PBXPublicIP
		}
		switch {
		case cfg.PBXPublicIP == "":
			add("public_ip", "Call audio address", "warn", "PBX_PUBLIC_IP is not visible to this check.", "Make sure PBX_PUBLIC_IP in Coolify is the server's current public IPv4.")
		case err != nil || len(v4) == 0:
			add("public_ip", "Call audio address", "warn", "Could not look up "+domain+".", "Check the DNS A record for "+domain+".")
		case !matched:
			add("public_ip", "Call audio address", "fail", fmt.Sprintf("PBX_PUBLIC_IP is %s but %s points to %s. Calls will ring with no sound.", cfg.PBXPublicIP, domain, strings.Join(v4, ", ")), "If the server's IP changed, set PBX_PUBLIC_IP to the new IP in Coolify and redeploy. Use an Elastic IP so it stops changing.")
		default:
			add("public_ip", "Call audio address", "ok", cfg.PBXPublicIP+" matches "+domain+".", "")
		}
	}
	var state, assigned string
	if cfg.TwilioBootstrapUser != "" {
		h.deps.DB.Conn().QueryRowContext(ctx, "SELECT a.state,a.number FROM sip_accounts a JOIN devices d ON d.id=a.device_id WHERE d.username=?", cfg.TwilioBootstrapUser).Scan(&state, &assigned)
	}
	switch {
	case state == "ready":
		add("phone_user", "Phone user set up", "ok", cfg.TwilioBootstrapUser+" on "+assigned, "")
	case cfg.PhonePassword == "":
		add("phone_user", "Phone user set up", "fail", "GOSIP_PHONE_PASSWORD is empty.", "Set a phone password in Coolify and redeploy.")
	default:
		add("phone_user", "Phone user set up", "warn", "Not ready yet (retrying automatically).", "Fix any red items above; the server retries every few minutes.")
	}
	online := false
	if state == "ready" {
		if req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil); err == nil {
			for _, connected := range NewDeviceHandler(h.deps).pbxRegistrations(req) {
				online = online || connected
			}
		}
	}
	if online {
		add("registered", "Linphone connected", "ok", "A phone is signed in.", "")
	} else {
		add("registered", "Linphone connected", "warn", "No phone signed in right now.", "Follow Phone Setup to sign in Linphone.")
	}
	return checks
}
