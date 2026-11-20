-- Count AI replies per conversation so one contact cannot run up AI/SMS costs.
ALTER TABLE automation_contacts ADD COLUMN ai_turns INTEGER NOT NULL DEFAULT 0;
