package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Linphone remote provisioning: Linphone's "Fetch remote configuration"
// downloads this XML and creates the account with the right server, port,
// TLS and SRTP. The phone form cannot set a non-standard port by itself.
// The file holds no password (Linphone asks for it) and is served only at an
// unguessable URL derived from the PBX secret.

func linphoneProvisioningToken(secret string) string {
	if secret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("linphone-remote-provisioning"))
	return hex.EncodeToString(mac.Sum(nil))[:32]
}

// linphoneProvisioningURL is shown to administrators on the Phone Setup page.
func linphoneProvisioningURL(deps *Dependencies) string {
	token := linphoneProvisioningToken(deps.Config.PBXSecret)
	base := strings.TrimRight(deps.Config.PublicURL, "/")
	if token == "" || base == "" {
		return ""
	}
	return base + "/api/linphone/" + token + ".xml"
}

// LinphoneConfig serves GET /api/linphone/{token}.xml.
func (h *BusinessHandler) LinphoneConfig(w http.ResponseWriter, r *http.Request) {
	want := linphoneProvisioningToken(h.deps.Config.PBXSecret)
	got := strings.TrimSuffix(chi.URLParam(r, "token"), ".xml")
	if want == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		http.NotFound(w, r)
		return
	}
	username := h.deps.Config.TwilioBootstrapUser
	domain := phoneServer(h.deps.Config.PBXSIPDomain)
	if username == "" || domain == "" {
		http.Error(w, "Phone user is not configured", http.StatusServiceUnavailable)
		return
	}
	proxy := fmt.Sprintf("<sip:%s:%d;transport=tls>", domain, phoneSIPPort(h.deps.Config.PBXSIPPort))
	identity := fmt.Sprintf("\"%s\" <sip:%s@%s>", username, username, domain)
	e := html.EscapeString
	body := `<?xml version="1.0" encoding="UTF-8"?>
<config xmlns="http://www.linphone.org/xsds/lpconfig.xsd" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.linphone.org/xsds/lpconfig.xsd lpconfig.xsd">
  <section name="proxy_0">
    <entry name="reg_identity" overwrite="true">` + e(identity) + `</entry>
    <entry name="reg_proxy" overwrite="true">` + e(proxy) + `</entry>
    <entry name="reg_route" overwrite="true">` + e(proxy) + `</entry>
    <entry name="reg_sendregister" overwrite="true">1</entry>
    <entry name="reg_expires" overwrite="true">300</entry>
    <entry name="publish" overwrite="true">0</entry>
    <entry name="avpf" overwrite="true">0</entry>
  </section>
  <section name="auth_info_0">
    <entry name="username" overwrite="true">` + e(username) + `</entry>
    <entry name="realm" overwrite="true">gosip</entry>
    <entry name="domain" overwrite="true">` + e(domain) + `</entry>
  </section>
  <section name="sip">
    <entry name="media_encryption" overwrite="true">srtp</entry>
    <entry name="media_encryption_mandatory" overwrite="true">1</entry>
  </section>
</config>
`
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(body))
}
