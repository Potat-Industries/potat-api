-- potat-api schema
-- Run against the database before starting the API:
--   docker exec -i potat-api-postgres-1 psql -U potat -d potat < schema.sql
--
-- NOTE: the haste service requires the pgzstd Postgres extension
-- (https://github.com/grahamedgecombe/pgzstd).  The standard postgres:16
-- Docker image does not include it, so leave haste.enabled=false in
-- config.docker.json unless you build a custom image with the extension.

-- ---------------------------------------------------------------------------
-- Core user tables  (required for /login and all user-facing routes)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS users (
    user_id    SERIAL      PRIMARY KEY,
    username   TEXT        NOT NULL UNIQUE,
    display    TEXT        NOT NULL DEFAULT '',
    first_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    level      INT         NOT NULL DEFAULT 1,
    settings   JSONB       NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS user_connections (
    user_id           INT         NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    platform_id       TEXT        NOT NULL,
    platform_username TEXT        NOT NULL DEFAULT '',
    platform_display  TEXT        NOT NULL DEFAULT '',
    platform_pfp      TEXT        NOT NULL DEFAULT '',
    platform          TEXT        NOT NULL,
    platform_metadata JSONB       NOT NULL DEFAULT '{}',
    PRIMARY KEY (user_id, platform, platform_id)
);

CREATE TABLE IF NOT EXISTS connection_oauth (
    platform_id   TEXT        NOT NULL,
    platform      TEXT        NOT NULL,
    access_token  TEXT        NOT NULL,
    refresh_token TEXT        NOT NULL DEFAULT '',
    scope         TEXT[]      NOT NULL DEFAULT '{}',
    expires_in    INT         NOT NULL DEFAULT 0,
    added_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (platform_id, platform)
);

-- ---------------------------------------------------------------------------
-- Channel tables
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS channels (
    channel_id   TEXT        NOT NULL,
    username     TEXT        NOT NULL,
    joined_at    TIMESTAMPTZ,
    added_by     JSONB       NOT NULL DEFAULT '[]',
    platform     TEXT        NOT NULL DEFAULT 'TWITCH',
    settings     JSONB       NOT NULL DEFAULT '{}',
    editors      TEXT[]      NOT NULL DEFAULT '{}',
    ambassadors  TEXT[]      NOT NULL DEFAULT '{}',
    meta         JSONB       NOT NULL DEFAULT '{}',
    state        TEXT        NOT NULL DEFAULT 'JOINED',
    PRIMARY KEY (channel_id, platform)
);

CREATE TABLE IF NOT EXISTS blocks (
    id         SERIAL PRIMARY KEY,
    user_id    INT    NOT NULL,
    block_id   INT    NOT NULL DEFAULT 0,
    channel_id TEXT   NOT NULL,
    block_type TEXT   NOT NULL DEFAULT 'USER',
    block_data TEXT   NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS custom_channel_commands (
    command_id      SERIAL      PRIMARY KEY,
    user_id         INT         NOT NULL,
    channel_id      TEXT        NOT NULL,
    name            TEXT,
    user_trigger_ids TEXT[]     NOT NULL DEFAULT '{}',
    user_ignore_ids  TEXT[]     NOT NULL DEFAULT '{}',
    trigger         TEXT        NOT NULL,
    response        TEXT        NOT NULL DEFAULT '',
    run_command     TEXT,
    active          BOOLEAN     NOT NULL DEFAULT TRUE,
    active_online   BOOLEAN     NOT NULL DEFAULT TRUE,
    active_offline  BOOLEAN     NOT NULL DEFAULT TRUE,
    reply           BOOLEAN     NOT NULL DEFAULT FALSE,
    whisper         BOOLEAN     NOT NULL DEFAULT FALSE,
    announce        BOOLEAN     NOT NULL DEFAULT FALSE,
    cooldown        INT         NOT NULL DEFAULT 5,
    delay           INT         NOT NULL DEFAULT 0,
    use_count       INT         NOT NULL DEFAULT 0,
    created         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    modified        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    platform        TEXT        NOT NULL DEFAULT 'TWITCH',
    help            TEXT
);

CREATE TABLE IF NOT EXISTS command_settings (
    channel_id        TEXT    NOT NULL,
    command           TEXT    NOT NULL,
    permission        TEXT    NOT NULL DEFAULT 'NONE',
    users_blacklisted TEXT[]  NOT NULL DEFAULT '{}',
    users_whitelisted TEXT[]  NOT NULL DEFAULT '{}',
    custom_cooldown   INT     NOT NULL DEFAULT 0,
    channel_usage     INT     NOT NULL DEFAULT 0,
    is_enabled        BOOLEAN NOT NULL DEFAULT TRUE,
    offline_only      BOOLEAN NOT NULL DEFAULT FALSE,
    silent_errors     BOOLEAN NOT NULL DEFAULT FALSE,
    allow_bots        BOOLEAN NOT NULL DEFAULT FALSE,
    platform          TEXT    NOT NULL DEFAULT 'TWITCH',
    PRIMARY KEY (channel_id, command, platform)
);

CREATE TABLE IF NOT EXISTS channel_command_usage (
    channel_id    TEXT   PRIMARY KEY,
    channel_usage BIGINT NOT NULL DEFAULT 0
);

-- ---------------------------------------------------------------------------
-- Potato game tables
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS potatoes (
    user_id         INT  PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    potato_count    INT  NOT NULL DEFAULT 0,
    potato_prestige INT  NOT NULL DEFAULT 0,
    potato_rank     INT  NOT NULL DEFAULT 0,
    tax_multiplier  INT  NOT NULL DEFAULT 0,
    first_seen      TEXT NOT NULL DEFAULT '',
    stole_from      TEXT,
    stole_amount    INT,
    trampled_by     TEXT
);

CREATE TABLE IF NOT EXISTS potato_analytics (
    user_id               INT  PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    average_response_time TEXT NOT NULL DEFAULT '0',
    eat_count             INT  NOT NULL DEFAULT 0,
    harvest_count         INT  NOT NULL DEFAULT 0,
    stolen_count          INT  NOT NULL DEFAULT 0,
    theft_count           INT  NOT NULL DEFAULT 0,
    trampled_count        INT  NOT NULL DEFAULT 0,
    trample_count         INT  NOT NULL DEFAULT 0,
    cdr_count             INT  NOT NULL DEFAULT 0,
    quiz_count            INT  NOT NULL DEFAULT 0,
    quiz_complete_count   INT  NOT NULL DEFAULT 0,
    guard_buy_count       INT  NOT NULL DEFAULT 0,
    fertilizer_buy_count  INT  NOT NULL DEFAULT 0,
    cdr_buy_count         INT  NOT NULL DEFAULT 0,
    new_quiz_buy_count    INT  NOT NULL DEFAULT 0,
    gamble_win_count      INT  NOT NULL DEFAULT 0,
    gamble_loss_count     INT  NOT NULL DEFAULT 0,
    gamble_wins_total     INT  NOT NULL DEFAULT 0,
    gamble_losses_total   INT  NOT NULL DEFAULT 0,
    duel_win_count        INT  NOT NULL DEFAULT 0,
    duel_loss_count       INT  NOT NULL DEFAULT 0,
    duel_wins_amount      INT  NOT NULL DEFAULT 0,
    duel_losses_amount    INT  NOT NULL DEFAULT 0,
    duel_caught_losses    INT  NOT NULL DEFAULT 0,
    average_response_count INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS potato_settings (
    user_id     INT     PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    not_verbose BOOLEAN NOT NULL DEFAULT FALSE
);

-- ---------------------------------------------------------------------------
-- GPT usage table (reset by hourly/daily/weekly cron loops)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS gpt_usage (
    user_id      INT PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    hourly_usage INT NOT NULL DEFAULT 0,
    daily_usage  INT NOT NULL DEFAULT 0,
    weekly_usage INT NOT NULL DEFAULT 0
);

-- ---------------------------------------------------------------------------
-- Service tables (auto-created on startup when the service is enabled,
-- included here so they can be pre-created for a fresh database)
-- ---------------------------------------------------------------------------

-- Requires pgzstd extension — omit if not using the haste service.
-- CREATE EXTENSION IF NOT EXISTS pgzstd;
-- CREATE TABLE IF NOT EXISTS haste (
--     key          CHAR(32)    UNIQUE NOT NULL,
--     content      BYTEA       NOT NULL,
--     access_count INT         NOT NULL DEFAULT 1,
--     source       TEXT        NOT NULL DEFAULT 'potatbotat',
--     timestamp    TIMESTAMPTZ NOT NULL DEFAULT NOW()
-- );

CREATE TABLE IF NOT EXISTS url_redirects (
    key VARCHAR(9)   PRIMARY KEY,
    url VARCHAR(500) NOT NULL
);

CREATE TABLE IF NOT EXISTS file_store (
    key        VARCHAR(50)  PRIMARY KEY,
    file       BYTEA        NOT NULL,
    file_name  VARCHAR(50),
    mime_type  VARCHAR(50)  NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- command_settings — add nullable columns and new fields
-- (ALTER TABLE is idempotent via IF NOT EXISTS)
-- ---------------------------------------------------------------------------

ALTER TABLE command_settings
    ALTER COLUMN permission       DROP NOT NULL,
    ALTER COLUMN offline_only     DROP NOT NULL,
    ALTER COLUMN custom_cooldown  DROP NOT NULL,
    ALTER COLUMN users_blacklisted DROP NOT NULL,
    ALTER COLUMN users_whitelisted DROP NOT NULL,
    ALTER COLUMN allow_bots       DROP NOT NULL;

ALTER TABLE command_settings
    ADD COLUMN IF NOT EXISTS platform           TEXT    NOT NULL DEFAULT 'TWITCH',
    ADD COLUMN IF NOT EXISTS ambassador_granted BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE command_settings DROP CONSTRAINT IF EXISTS command_settings_pkey;
ALTER TABLE command_settings ADD PRIMARY KEY (channel_id, command, platform);

ALTER TABLE channels DROP CONSTRAINT IF EXISTS channels_pkey;
ALTER TABLE channels ADD PRIMARY KEY (channel_id, platform);

-- ---------------------------------------------------------------------------
-- Reminders
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS reminders (
    reminder_id  SERIAL       PRIMARY KEY,
    user_id      VARCHAR(64)  NOT NULL,
    recipient_id VARCHAR(64)  NOT NULL,
    channel_id   VARCHAR(64)  NOT NULL,
    message      VARCHAR(500) NOT NULL DEFAULT '',
    ready_at     TIMESTAMPTZ,
    set_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    afk_withheld BOOLEAN      NOT NULL DEFAULT FALSE,
    status       TEXT         NOT NULL DEFAULT 'PENDING',
    platform     TEXT         NOT NULL DEFAULT 'TWITCH',
    sent_at      TIMESTAMPTZ,
    type         TEXT         NOT NULL DEFAULT 'NEXT'
);

CREATE INDEX IF NOT EXISTS idx_reminders_recipient ON reminders (recipient_id, platform, status);

-- ---------------------------------------------------------------------------
-- Website JWT sessions
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS website_jwt (
    token   TEXT NOT NULL,
    user_id INT  NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    PRIMARY KEY (token)
);
