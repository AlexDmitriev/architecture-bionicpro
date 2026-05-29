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

CREATE SCHEMA IF NOT EXISTS olap;

CREATE TABLE IF NOT EXISTS telemetry_events (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    device_id VARCHAR(128),
    event_type VARCHAR(64) NOT NULL,
    value NUMERIC(10, 2),
    battery_pct NUMERIC(5, 2),
    is_error BOOLEAN NOT NULL DEFAULT FALSE,
    session_seconds INTEGER NOT NULL DEFAULT 0,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_telemetry_events_user_recorded
    ON telemetry_events (user_id, recorded_at DESC);

CREATE TABLE IF NOT EXISTS olap.stg_crm_customers (
    customer_id INTEGER PRIMARY KEY,
    customer_name VARCHAR(100),
    user_id VARCHAR(255) NOT NULL,
    country VARCHAR(100),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS olap.stg_telemetry_agg (
    user_id VARCHAR(255) PRIMARY KEY,
    sessions_count INTEGER NOT NULL DEFAULT 0,
    total_usage_minutes NUMERIC(12, 2) NOT NULL DEFAULT 0,
    movements_count INTEGER NOT NULL DEFAULT 0,
    avg_battery_pct NUMERIC(5, 2),
    errors_count INTEGER NOT NULL DEFAULT 0,
    report_period_start DATE NOT NULL,
    report_period_end DATE NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS olap.mart_user_report (
    user_id VARCHAR(255) PRIMARY KEY,
    customer_id INTEGER,
    customer_name VARCHAR(100),
    country VARCHAR(100),
    report_period_start DATE NOT NULL,
    report_period_end DATE NOT NULL,
    sessions_count INTEGER NOT NULL DEFAULT 0,
    total_usage_minutes NUMERIC(12, 2) NOT NULL DEFAULT 0,
    movements_count INTEGER NOT NULL DEFAULT 0,
    avg_battery_pct NUMERIC(5, 2),
    errors_count INTEGER NOT NULL DEFAULT 0,
    report_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mart_user_report_customer_period
    ON olap.mart_user_report (customer_id, report_period_end DESC);
