-- Per-contact state for n8n automation: SMS opt-outs and AI hand-over pauses.
CREATE TABLE IF NOT EXISTS automation_contacts (
    number TEXT PRIMARY KEY,
    opted_out INTEGER NOT NULL DEFAULT 0,
    ai_paused_until DATETIME,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
