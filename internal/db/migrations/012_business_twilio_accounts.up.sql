ALTER TABLE sip_accounts ADD COLUMN twilio_account_sid TEXT NOT NULL DEFAULT '';
UPDATE sip_accounts SET twilio_account_sid=COALESCE((SELECT value FROM config WHERE key='twilio_account_sid'),'') WHERE twilio_account_sid='';
