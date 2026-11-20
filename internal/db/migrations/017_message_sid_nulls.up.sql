-- Empty provider SIDs collide on the UNIQUE index; store missing SIDs as NULL.
UPDATE messages SET message_sid = NULL WHERE message_sid = '';
