package api

import (
	"context"
	"database/sql"
	"sync"

	"github.com/rohitkumar-co-in/leadomi-sip/internal/config"
	"github.com/rohitkumar-co-in/leadomi-sip/internal/db"
	"github.com/rohitkumar-co-in/leadomi-sip/internal/models"
	"github.com/rohitkumar-co-in/leadomi-sip/internal/twilio"
	"github.com/rohitkumar-co-in/leadomi-sip/pkg/sip"
)

// Dependencies holds all dependencies for API handlers
type Dependencies struct {
	DB       *db.DB
	SIP      *sip.Server
	Twilio   TwilioClient
	Notifier Notifier
	Config   *config.Config
	twilioMu sync.Mutex
	twilioClients map[string]TwilioClient
}

// TwilioClient interface for Twilio operations
type TwilioClient interface {
	// SMS/MMS Operations
	SendSMS(from, to, body string, mediaURLs []string) (string, error)
	SendSMSWithCallback(from, to, body string, mediaURLs []string, statusCallback string) (string, error)
	GetMessage(ctx context.Context, messageSID string) (*twilio.TwilioMessage, error)
	ListMessages(ctx context.Context, from, to string, limit int) ([]*twilio.TwilioMessage, error)
	DeleteMessage(ctx context.Context, messageSID string) error
	CancelMessage(ctx context.Context, messageSID string) error
	ResendMessage(ctx context.Context, originalSID string) (string, error)
	GetMediaURLs(ctx context.Context, messageSID string) ([]string, error)

	// Voice Operations
	RequestTranscription(recordingSID string, voicemailID int64) error

	// Account Operations
	UpdateCredentials(accountSID, authToken string)
	IsHealthy() bool
	ListIncomingPhoneNumbers(ctx context.Context) ([]twilio.IncomingPhoneNumber, error)

	// SIP Trunk Operations
	ListSIPTrunks(ctx context.Context) ([]*twilio.SIPTrunk, error)
	CreateSIPTrunk(ctx context.Context, friendlyName string, secure bool) (*twilio.SIPTrunk, error)
	GetSIPTrunk(ctx context.Context, trunkSID string) (*twilio.SIPTrunk, error)
	UpdateSIPTrunk(ctx context.Context, trunkSID string, secure bool, friendlyName string) error
	DeleteSIPTrunk(ctx context.Context, trunkSID string) error
	GetTrunkTLSStatus(ctx context.Context, trunkSID string) (*twilio.TrunkTLSStatus, error)
	EnableTLSForTrunk(ctx context.Context, trunkSID string) error
	DisableTLSForTrunk(ctx context.Context, trunkSID string) error
	MigrateToSecureOrigination(ctx context.Context, trunkSID string) error
	EnsureTrunkFullySecure(ctx context.Context, trunkSID string) error
	SetOriginationURI(ctx context.Context, trunkSID, sipURI string, priority, weight int) error
	SetSecureOriginationURI(ctx context.Context, trunkSID, sipURI string, priority, weight int) error
	AssignPhoneNumberToTrunk(ctx context.Context, trunkSID, phoneNumberSID string) error

	// Voice Operations
	MakeCall(from, to, url string) (string, error)
}

// Notifier interface for sending notifications
type Notifier interface {
	SendVoicemailNotification(voicemail *models.Voicemail) error
	SendSMSNotification(message *models.Message) error
	SendEmail(to, subject, body string) error
	SendPush(title, message string) error
}

// NewDependencies creates a new Dependencies instance
func NewDependencies(cfg *config.Config, database *db.DB, sipServer *sip.Server, twilio TwilioClient, notifier Notifier) *Dependencies {
	clients := make(map[string]TwilioClient)
	if sid, _ := cfg.TwilioCredentials(); sid != "" && twilio != nil {
		clients[sid] = twilio
	}
	return &Dependencies{
		DB:       database,
		SIP:      sipServer,
		Twilio:   twilio,
		Notifier: notifier,
		Config:   cfg,
		twilioClients: clients,
	}
}

// TwilioForAccount returns a client scoped to the requested account SID.
func (d *Dependencies) TwilioForAccount(accountSID string) TwilioClient {
	if d == nil || d.Config == nil {
		return nil
	}
	if accountSID == "" {
		accountSID, _ = d.Config.TwilioCredentials()
	}
	primarySID, _ := d.Config.TwilioCredentials()
	if d.Twilio != nil && (accountSID == "" || accountSID == primarySID) {
		return d.Twilio
	}
	d.twilioMu.Lock()
	defer d.twilioMu.Unlock()
	if d.twilioClients == nil {
		d.twilioClients = make(map[string]TwilioClient)
	}
	if client := d.twilioClients[accountSID]; client != nil {
		return client
	}
	sid, token := d.Config.TwilioCredentialsFor(accountSID)
	if sid == "" || token == "" {
		return nil
	}
	accountConfig := &config.Config{TwilioAccountSID: sid, TwilioAuthToken: token}
	client := twilio.NewClient(accountConfig)
	d.twilioClients[sid] = client
	return client
}

func (d *Dependencies) TwilioForAccountForDID(ctx context.Context, didID int64) TwilioClient {
	if d == nil || d.DB == nil {
		return nil
	}
	var accountSID string
	if err := d.DB.Conn().QueryRowContext(ctx, "SELECT a.twilio_account_sid FROM sip_accounts a JOIN dids d ON d.number=a.number WHERE d.id=?", didID).Scan(&accountSID); err != nil {
		if err != sql.ErrNoRows {
			return nil
		}
		return d.TwilioForAccount("")
	}
	return d.TwilioForAccount(accountSID)
}
