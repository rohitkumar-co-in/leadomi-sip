-- Stop users assigned through a non-primary Twilio account before removing the
-- account mapping. Their phone and number records remain available for review.
UPDATE sip_accounts
SET enabled=0,state='error',updated_at=CURRENT_TIMESTAMP
WHERE twilio_account_sid<>''
  AND EXISTS (SELECT 1 FROM config WHERE key='pbx_primary_twilio_account_sid')
  AND twilio_account_sid<>(SELECT value FROM config WHERE key='pbx_primary_twilio_account_sid');

-- Rebuild the PBX map using only enabled assignments so disabled secondary
-- users are not left configured for outbound dialing.
INSERT INTO config(key,value)
SELECT 'pbx_device_numbers', COALESCE((
  SELECT json_group_object(d.username,a.number)
  FROM sip_accounts a JOIN devices d ON d.id=a.device_id
  WHERE a.enabled=1 AND a.state='ready' AND a.number<>''
), '{}')
ON CONFLICT(key) DO UPDATE SET value=excluded.value;

DELETE FROM config WHERE key IN ('pbx_primary_twilio_account_sid','pbx_device_accounts');
DELETE FROM config WHERE key GLOB 'pbx_twilio_domain_sid_*'
   OR key GLOB 'pbx_twilio_credential_list_sid_*'
   OR key GLOB 'pbx_twilio_domain_name_*';

ALTER TABLE sip_accounts DROP COLUMN twilio_account_sid;
