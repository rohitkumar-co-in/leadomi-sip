package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var twilioAccountSIDPattern = regexp.MustCompile(`^AC[a-fA-F0-9]{32}$`)

func (c *Config) twilioAccountsPath() string {
	return filepath.Join(c.DataDir, "twilio-accounts.json")
}

// LoadTwilioAccounts loads secondary API tokens from a private data-volume file.
// Account credentials are kept outside SQLite and are never exposed to clients.
func (c *Config) LoadTwilioAccounts() error {
	file, err := os.Open(c.twilioAccountsPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open secondary Twilio credentials: %w", err)
	}
	defer file.Close()
	if err := file.Chmod(0600); err != nil {
		return fmt.Errorf("protect secondary Twilio credentials: %w", err)
	}
	var accounts map[string]string
	if err := json.NewDecoder(file).Decode(&accounts); err != nil {
		return errors.New("invalid secondary Twilio credentials file")
	}
	for sid, token := range accounts {
		if !twilioAccountSIDPattern.MatchString(sid) || strings.TrimSpace(token) == "" || strings.ContainsAny(token, "\r\n\x00") {
			return errors.New("invalid secondary Twilio account entry")
		}
	}
	c.twilioMu.Lock()
	c.TwilioAccounts = accounts
	if c.TwilioAccounts == nil {
		c.TwilioAccounts = make(map[string]string)
	}
	c.twilioMu.Unlock()
	return nil
}

// TwilioCredentialsFor returns the token for a specific account. An empty SID
// selects the configured primary account for backward compatibility.
func (c *Config) TwilioCredentialsFor(accountSID string) (string, string) {
	if c == nil {
		return "", ""
	}
	c.twilioMu.RLock()
	defer c.twilioMu.RUnlock()
	if accountSID == "" || accountSID == c.TwilioAccountSID {
		return c.TwilioAccountSID, c.TwilioAuthToken
	}
	return accountSID, c.TwilioAccounts[accountSID]
}

// TwilioAccountSIDs returns configured account identifiers without secrets.
func (c *Config) TwilioAccountSIDs() []string {
	if c == nil {
		return nil
	}
	c.twilioMu.RLock()
	defer c.twilioMu.RUnlock()
	ids := make([]string, 0, len(c.TwilioAccounts)+1)
	if c.TwilioAccountSID != "" && c.TwilioAuthToken != "" {
		ids = append(ids, c.TwilioAccountSID)
	}
	for sid, token := range c.TwilioAccounts {
		if token != "" && sid != c.TwilioAccountSID {
			ids = append(ids, sid)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		if ids[i] == c.TwilioAccountSID {
			return true
		}
		if ids[j] == c.TwilioAccountSID {
			return false
		}
		return ids[i] < ids[j]
	})
	return ids
}

// SaveTwilioAccount adds or rotates a secondary account token atomically.
func (c *Config) SaveTwilioAccount(accountSID, authToken string) error {
	if c == nil || !twilioAccountSIDPattern.MatchString(accountSID) || strings.TrimSpace(authToken) == "" || strings.ContainsAny(authToken, "\r\n\x00") {
		return errors.New("a valid Twilio Account SID and Auth Token are required")
	}
	c.twilioMu.Lock()
	defer c.twilioMu.Unlock()
	if accountSID == c.TwilioAccountSID {
		return errors.New("use the primary Twilio credential settings for this account")
	}
	accounts := make(map[string]string, len(c.TwilioAccounts)+1)
	for sid, token := range c.TwilioAccounts {
		accounts[sid] = token
	}
	accounts[accountSID] = authToken
	if err := os.MkdirAll(c.DataDir, 0700); err != nil {
		return fmt.Errorf("create credentials directory: %w", err)
	}
	temp, err := os.CreateTemp(c.DataDir, ".twilio-accounts-*")
	if err != nil {
		return fmt.Errorf("create secondary credentials file: %w", err)
	}
	path := temp.Name()
	defer os.Remove(path)
	defer temp.Close()
	if err := temp.Chmod(0600); err != nil {
		return fmt.Errorf("protect secondary credentials file: %w", err)
	}
	if err := json.NewEncoder(temp).Encode(accounts); err != nil {
		return fmt.Errorf("write secondary credentials file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync secondary credentials file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close secondary credentials file: %w", err)
	}
	if err := os.Rename(path, c.twilioAccountsPath()); err != nil {
		return fmt.Errorf("replace secondary credentials file: %w", err)
	}
	c.TwilioAccounts = accounts
	return nil
}
