-- AI replies only to contacts who called: a missed call opens a session.
ALTER TABLE automation_contacts ADD COLUMN ai_session_until DATETIME;
