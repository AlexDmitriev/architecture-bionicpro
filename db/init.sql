CREATE TABLE IF NOT EXISTS yandex_user_profiles (
    id SERIAL PRIMARY KEY,
    keycloak_sub VARCHAR(255) NOT NULL UNIQUE,
    yandex_id VARCHAR(64),
    login VARCHAR(255),
    email VARCHAR(255),
    first_name VARCHAR(255),
    last_name VARCHAR(255),
    display_name VARCHAR(255),
    avatar_url TEXT,
    raw_profile JSONB,
    consent_granted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_yandex_profiles_yandex_id ON yandex_user_profiles (yandex_id);
