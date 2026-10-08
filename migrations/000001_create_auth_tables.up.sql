CREATE TABLE auth_accounts (
    id UUID PRIMARY KEY,
    email VARCHAR(320) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    personal_name VARCHAR(200) NOT NULL,
    merchant_name VARCHAR(200) NOT NULL,
    onboarding_status VARCHAR(20) NOT NULL CHECK (onboarding_status IN ('PENDING', 'COMPLETE')),
    backend_user_id BIGINT,
    merchant_id BIGINT,
    failed_logins INTEGER NOT NULL DEFAULT 0 CHECK (failed_logins >= 0),
    login_locked_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_auth_accounts_onboarding_status_created_at
    ON auth_accounts (onboarding_status, created_at);

CREATE TABLE auth_sessions (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL UNIQUE REFERENCES auth_accounts(id) ON DELETE CASCADE,
    current_refresh_hash CHAR(64) NOT NULL,
    previous_refresh_hash CHAR(64),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_auth_sessions_expires_at ON auth_sessions (expires_at);

CREATE TABLE auth_refresh_tokens (
    token_hash CHAR(64) PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES auth_accounts(id) ON DELETE CASCADE,
    session_id UUID NOT NULL REFERENCES auth_sessions(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ
);

CREATE INDEX idx_auth_refresh_tokens_account_used_expiry
    ON auth_refresh_tokens (account_id, used_at, expires_at);
