package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rohitkumar-co-in/leadomi-sip/internal/config"
	"github.com/rohitkumar-co-in/leadomi-sip/internal/models"
)

func dialerTestConfig() *config.Config {
	return &config.Config{TwilioAccountSID: "AC-test", TwilioAuthToken: "test-token", OutboundCallerID: "+447575746354", PublicURL: "https://sip.example.test",
		TwilioAPIKeySID: "SKtest", TwilioAPIKeySecret: "key-secret", TwilioTwiMLAppSID: "AP0123456789abcdef0123456789abcdef"}
}

func TestDialerAccessToken(t *testing.T) {
	cfg := dialerTestConfig()
	token, err := dialerAccessToken(cfg, time.Unix(1000, 0), time.Hour)
	parts := strings.Split(token, ".")
	if err != nil || len(parts) != 3 {
		t.Fatalf("bad token %q %v", token, err)
	}
	mac := hmac.New(sha256.New, []byte("key-secret"))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if base64.RawURLEncoding.EncodeToString(mac.Sum(nil)) != parts[2] {
		t.Fatal("token signature is wrong")
	}
	raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims struct {
		Iss, Sub string
		Exp      int64
		Grants   struct {
			Identity string
			Voice    struct {
				Outgoing struct {
					ApplicationSID string `json:"application_sid"`
				}
			}
		}
	}
	if json.Unmarshal(raw, &claims) != nil || claims.Iss != "SKtest" || claims.Sub != "AC-test" || claims.Exp != 4600 ||
		claims.Grants.Identity != dialerIdentity || claims.Grants.Voice.Outgoing.ApplicationSID != cfg.TwilioTwiMLAppSID {
		t.Fatalf("bad claims: %s", raw)
	}
}

func TestDialerBrowserCall(t *testing.T) {
	setup := setupTestAPI(t)
	cfg := dialerTestConfig()
	h := NewWebhookHandler(&Dependencies{Config: cfg, DB: setup.DB})
	if err := setup.DB.DIDs.Create(context.Background(), &models.DID{Number: cfg.OutboundCallerID, VoiceEnabled: true}); err != nil {
		t.Fatal(err)
	}
	call := func(from, to string) string {
		rr := httptest.NewRecorder()
		h.VoiceBrowser(rr, signedSoftphoneRequest(cfg.PublicURL+"/api/webhooks/voice/browser", url.Values{"AccountSid": {"AC-test"}, "CallSid": {"CA-" + to}, "From": {from}, "To": {to}}))
		return rr.Body.String()
	}
	if body := call("client:"+dialerIdentity, "+441234567890"); !strings.Contains(body, `callerId="+447575746354"`) || !strings.Contains(body, "<Number>+441234567890</Number>") {
		t.Fatalf("browser call not bridged: %s", body)
	}
	if body := call("client:someone-else", "+441234567891"); strings.Contains(body, "<Dial") {
		t.Fatalf("foreign client must not dial: %s", body)
	}
	if body := call("client:"+dialerIdentity, "07700"); strings.Contains(body, "<Dial") {
		t.Fatalf("non E.164 must not dial: %s", body)
	}
}
