-- ─────────────────────────────────────────────
-- GiraffeMail Archive — initial schema
-- ─────────────────────────────────────────────

-- ── Auth ──────────────────────────────────────

CREATE TABLE IF NOT EXISTS users (
    id           TEXT    PRIMARY KEY,
    email        TEXT    UNIQUE NOT NULL,
    password_hash TEXT   NOT NULL,
    full_name    TEXT    NOT NULL,
    role         TEXT    NOT NULL DEFAULT 'user' CHECK(role IN ('admin', 'user')),
    is_active    INTEGER NOT NULL DEFAULT 1,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Opaque bearer tokens stored as SHA-256 hash, 30-day expiry.
CREATE TABLE IF NOT EXISTS user_tokens (
    id          TEXT     PRIMARY KEY,
    user_id     TEXT     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  TEXT     UNIQUE NOT NULL,
    expires_at  DATETIME NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_user_tokens_hash   ON user_tokens(token_hash, expires_at);
CREATE INDEX IF NOT EXISTS idx_user_tokens_user   ON user_tokens(user_id);

CREATE TABLE IF NOT EXISTS user_otps (
    id          TEXT     PRIMARY KEY,
    identifier  TEXT     NOT NULL,
    code        TEXT     NOT NULL,
    type        TEXT     NOT NULL CHECK(type IN ('registration', 'forgot_password')),
    is_used     INTEGER  NOT NULL DEFAULT 0,
    expires_at  DATETIME NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_user_otps_lookup ON user_otps(identifier, type, is_used, expires_at);

-- ── Mail accounts ─────────────────────────────

CREATE TABLE IF NOT EXISTS mail_accounts (
    id                    TEXT     PRIMARY KEY,
    name                  TEXT     NOT NULL,
    email_address         TEXT     NOT NULL,
    imap_host             TEXT     NOT NULL,
    imap_port             INTEGER  NOT NULL DEFAULT 993,
    imap_use_tls          INTEGER  NOT NULL DEFAULT 1,
    username              TEXT     NOT NULL,
    -- AES-256-GCM encrypted; nonce prepended as first 12 bytes, base64-encoded
    credentials_encrypted TEXT     NOT NULL,
    sync_enabled          INTEGER  NOT NULL DEFAULT 1,
    last_sync_at          DATETIME,
    created_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ── Mailboxes (IMAP folders) ──────────────────

CREATE TABLE IF NOT EXISTS mailboxes (
    id                  TEXT     PRIMARY KEY,
    account_id          TEXT     NOT NULL REFERENCES mail_accounts(id) ON DELETE CASCADE,
    name                TEXT     NOT NULL,   -- IMAP folder name, e.g. "INBOX"
    uid_validity        INTEGER  NOT NULL DEFAULT 0,
    highest_uid         INTEGER  NOT NULL DEFAULT 0,
    message_count       INTEGER  NOT NULL DEFAULT 0,
    total_size_bytes    INTEGER  NOT NULL DEFAULT 0,
    archived_count      INTEGER  NOT NULL DEFAULT 0,
    archived_size_bytes INTEGER  NOT NULL DEFAULT 0,
    last_indexed_at     DATETIME,
    last_archived_at    DATETIME,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(account_id, name)
);

CREATE INDEX IF NOT EXISTS idx_mailboxes_account ON mailboxes(account_id);

-- ── Messages ──────────────────────────────────

CREATE TABLE IF NOT EXISTS messages (
    id                    TEXT     PRIMARY KEY,
    account_id            TEXT     NOT NULL REFERENCES mail_accounts(id) ON DELETE CASCADE,
    mailbox_id            TEXT     NOT NULL REFERENCES mailboxes(id)     ON DELETE CASCADE,
    uid                   INTEGER  NOT NULL,
    message_id_header     TEXT,                       -- RFC822 Message-ID header (may be absent)
    subject               TEXT,
    sender_name           TEXT,
    sender_email          TEXT     NOT NULL DEFAULT '',
    recipients_json       TEXT     NOT NULL DEFAULT '[]', -- [{name,email}]
    date                  DATETIME,
    size_bytes            INTEGER  NOT NULL DEFAULT 0,
    flags_json            TEXT     NOT NULL DEFAULT '[]', -- ["\\Seen", "\\Answered", ...]
    has_attachments       INTEGER  NOT NULL DEFAULT 0,
    attachment_count      INTEGER  NOT NULL DEFAULT 0,

    -- Archive state — the golden rule fields.
    -- Only both non-NULL together means the message is safely archived.
    blob_sha256           TEXT,    -- NULL until blob written and verified
    archived_at           DATETIME,-- NULL until checksum confirmed
    deleted_from_server_at DATETIME,-- NULL while still on server

    body_preview          TEXT,    -- first 200 chars of plain-text body

    indexed_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(mailbox_id, uid)
);

CREATE INDEX IF NOT EXISTS idx_messages_account       ON messages(account_id);
CREATE INDEX IF NOT EXISTS idx_messages_mailbox       ON messages(mailbox_id);
CREATE INDEX IF NOT EXISTS idx_messages_sender        ON messages(sender_email);
CREATE INDEX IF NOT EXISTS idx_messages_date          ON messages(date);
CREATE INDEX IF NOT EXISTS idx_messages_size          ON messages(size_bytes);
CREATE INDEX IF NOT EXISTS idx_messages_archived      ON messages(archived_at);
CREATE INDEX IF NOT EXISTS idx_messages_blob          ON messages(blob_sha256);
CREATE INDEX IF NOT EXISTS idx_messages_deleted       ON messages(deleted_from_server_at);
-- composite for cleanup safety gate query
CREATE INDEX IF NOT EXISTS idx_messages_cleanup_gate  ON messages(archived_at, blob_sha256, deleted_from_server_at);
-- composite for incremental sync cursor
CREATE INDEX IF NOT EXISTS idx_messages_sync_cursor   ON messages(mailbox_id, uid);

-- ── FTS5 search index ─────────────────────────

-- Standalone FTS5 table (not content-linked to avoid trigger complexity).
-- Populated during Phase 2 (archive) when body text is available.
-- message_db_id links back to messages.id for joining.
CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
    message_db_id  UNINDEXED,
    subject,
    sender_name,
    sender_email,
    recipients_text,
    body_text,
    tokenize = 'porter unicode61'
);

-- ── Attachments ───────────────────────────────

-- Metadata only; raw bytes live inside the .eml.zst blob, extracted on demand.
CREATE TABLE IF NOT EXISTS attachments (
    id            TEXT     PRIMARY KEY,
    message_id    TEXT     NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    filename      TEXT     NOT NULL,
    content_type  TEXT     NOT NULL,
    size_bytes    INTEGER  NOT NULL DEFAULT 0,
    -- MIME part path used to extract on demand, e.g. "1.2" for multipart
    part_path     TEXT     NOT NULL,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_attachments_message ON attachments(message_id);

-- ── Cleanup jobs ──────────────────────────────

CREATE TABLE IF NOT EXISTS cleanup_jobs (
    id                  TEXT     PRIMARY KEY,
    name                TEXT     NOT NULL,
    account_id          TEXT     REFERENCES mail_accounts(id) ON DELETE SET NULL,
    filter_json         TEXT     NOT NULL DEFAULT '{}',
    action              TEXT     NOT NULL CHECK(action IN ('delete', 'move')),
    move_target_folder  TEXT,
    created_by          TEXT     REFERENCES users(id) ON DELETE SET NULL,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS cleanup_job_runs (
    id                  TEXT     PRIMARY KEY,
    job_id              TEXT     NOT NULL REFERENCES cleanup_jobs(id) ON DELETE CASCADE,
    status              TEXT     NOT NULL DEFAULT 'pending'
                                 CHECK(status IN ('pending', 'running', 'done', 'failed', 'cancelled')),
    total_candidates    INTEGER  NOT NULL DEFAULT 0,
    processed           INTEGER  NOT NULL DEFAULT 0,
    skipped_unarchived  INTEGER  NOT NULL DEFAULT 0,  -- messages that were not safely archived; never touched
    freed_bytes         INTEGER  NOT NULL DEFAULT 0,
    error_message       TEXT,
    started_at          DATETIME,
    finished_at         DATETIME,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_cleanup_runs_job ON cleanup_job_runs(job_id, status);

-- ── Company profile (single row) ─────────────

CREATE TABLE IF NOT EXISTS company_profile (
    id         INTEGER  PRIMARY KEY CHECK(id = 1),
    name       TEXT     NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO company_profile(id, name) VALUES (1, '');
