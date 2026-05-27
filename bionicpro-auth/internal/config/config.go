package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr              string
	KeycloakURL       string
	KeycloakPublicURL string
	KeycloakRealm     string
	KeycloakClientID  string
	KeycloakSecret    string
	RedirectURI       string
	FrontendURL       string
	ReportsAPIURL     string
	RedisAddr         string
	SessionTTL        time.Duration
	CookieName        string
	CookieSecure      bool
	EncryptionKey     []byte
	DatabaseURL       string
	YandexIdPAlias    string
}

func Load() Config {
	sessionMinutes := envInt("SESSION_TTL_MINUTES", 30)
	key := os.Getenv("TOKEN_ENCRYPTION_KEY")
	if len(key) < 32 {
		key = "dev-only-32-byte-encryption-key!!"
	}

	return Config{
		Addr:             env("ADDR", ":8081"),
		KeycloakURL:      env("KEYCLOAK_URL", "http://keycloak:8080"),
		KeycloakPublicURL: env(
			"KEYCLOAK_PUBLIC_URL",
			env("KEYCLOAK_URL", "http://keycloak:8080"),
		),
		KeycloakRealm:    env("KEYCLOAK_REALM", "reports-realm"),
		KeycloakClientID: env("KEYCLOAK_CLIENT_ID", "bionicpro-auth"),
		KeycloakSecret:   env("KEYCLOAK_CLIENT_SECRET", "bionicpro-auth-secret-change-me"),
		RedirectURI:      env("REDIRECT_URI", "http://localhost:8081/auth/callback"),
		FrontendURL:      env("FRONTEND_URL", "http://localhost:3000"),
		ReportsAPIURL:    env("REPORTS_API_URL", "http://localhost:8000"),
		RedisAddr:        env("REDIS_ADDR", "redis:6379"),
		SessionTTL:       time.Duration(sessionMinutes) * time.Minute,
		CookieName:       env("SESSION_COOKIE_NAME", "bionicpro_session"),
		CookieSecure:     envBool("COOKIE_SECURE", false),
		EncryptionKey:    []byte(key)[:32],
		DatabaseURL:      env("DATABASE_URL", "postgres://profiles:profiles@profiles_db:5432/profiles?sslmode=disable"),
		YandexIdPAlias:   env("YANDEX_IDP_ALIAS", "yandex"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
