ALTER TABLE mail_accounts ADD COLUMN owner_id TEXT REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_mail_accounts_owner ON mail_accounts(owner_id);

UPDATE mail_accounts
SET    owner_id = (SELECT id FROM users WHERE role = 'admin' ORDER BY created_at LIMIT 1)
WHERE  owner_id IS NULL
  AND  EXISTS (SELECT 1 FROM users WHERE role = 'admin');
