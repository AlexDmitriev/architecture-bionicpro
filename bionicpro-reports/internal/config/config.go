package config

import (
	"os"
	"strconv"
)

type Config struct {
	Addr          string
	ClickHouseDSN string
	KeycloakURL   string
	KeycloakRealm string
	S3Endpoint    string
	S3AccessKey   string
	S3SecretKey   string
	S3Bucket      string
	S3UseSSL      bool
	CDNBaseURL    string
	DataVersion   string
	CDNMaxAgeSec  int
}

func Load() Config {
	return Config{
		Addr:          env("ADDR", ":8000"),
		ClickHouseDSN: env("CLICKHOUSE_DSN", "clickhouse://clickhouse:9000?database=olap"),
		KeycloakURL:   env("KEYCLOAK_URL", "http://keycloak:8080"),
		KeycloakRealm: env("KEYCLOAK_REALM", "reports-realm"),
		S3Endpoint:    env("S3_ENDPOINT", "minio:9000"),
		S3AccessKey:   env("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey:   env("S3_SECRET_KEY", "minioadmin"),
		S3Bucket:      env("S3_BUCKET", "reports"),
		S3UseSSL:      envBool("S3_USE_SSL", false),
		CDNBaseURL:    env("CDN_BASE_URL", "http://localhost:8082/reports"),
		DataVersion:   env("REPORTS_DATA_VERSION", "v1"),
		CDNMaxAgeSec:  envInt("CDN_MAX_AGE_SECONDS", 3600),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		parsed, err := strconv.Atoi(v)
		if err == nil {
			return parsed
		}
	}
	return fallback
}
