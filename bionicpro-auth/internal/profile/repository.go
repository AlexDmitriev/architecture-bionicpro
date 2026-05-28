package profile

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("profile not found")

type Record struct {
	KeycloakSub      string
	YandexID         string
	Login            string
	Email            string
	FirstName        string
	LastName         string
	DisplayName      string
	AvatarURL        string
	RawProfile       json.RawMessage
	ConsentGrantedAt *time.Time
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) HasConsent(ctx context.Context, keycloakSub string) (bool, error) {
	var granted *time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT consent_granted_at FROM yandex_user_profiles WHERE keycloak_sub = $1`,
		keycloakSub,
	).Scan(&granted)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return granted != nil, nil
}

func (r *Repository) GetBySub(ctx context.Context, keycloakSub string) (*Record, error) {
	var rec Record
	err := r.pool.QueryRow(ctx, `
		SELECT keycloak_sub, yandex_id, login, email, first_name, last_name,
		       display_name, avatar_url, raw_profile, consent_granted_at
		FROM yandex_user_profiles WHERE keycloak_sub = $1`,
		keycloakSub,
	).Scan(
		&rec.KeycloakSub, &rec.YandexID, &rec.Login, &rec.Email,
		&rec.FirstName, &rec.LastName, &rec.DisplayName, &rec.AvatarURL,
		&rec.RawProfile, &rec.ConsentGrantedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *Repository) UpsertWithConsent(ctx context.Context, rec Record) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO yandex_user_profiles (
			keycloak_sub, yandex_id, login, email, first_name, last_name,
			display_name, avatar_url, raw_profile, consent_granted_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW(),NOW())
		ON CONFLICT (keycloak_sub) DO UPDATE SET
			yandex_id = EXCLUDED.yandex_id,
			login = EXCLUDED.login,
			email = EXCLUDED.email,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			display_name = EXCLUDED.display_name,
			avatar_url = EXCLUDED.avatar_url,
			raw_profile = EXCLUDED.raw_profile,
			consent_granted_at = NOW(),
			updated_at = NOW()`,
		rec.KeycloakSub, rec.YandexID, rec.Login, rec.Email,
		rec.FirstName, rec.LastName, rec.DisplayName, rec.AvatarURL, rec.RawProfile,
	)
	return err
}

func (r *Repository) UpsertWithoutConsent(ctx context.Context, rec Record) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO yandex_user_profiles (
			keycloak_sub, yandex_id, login, email, first_name, last_name,
			display_name, avatar_url, raw_profile, consent_granted_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NULL,NOW())
		ON CONFLICT (keycloak_sub) DO UPDATE SET
			yandex_id = EXCLUDED.yandex_id,
			login = EXCLUDED.login,
			email = EXCLUDED.email,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			display_name = EXCLUDED.display_name,
			avatar_url = EXCLUDED.avatar_url,
			raw_profile = EXCLUDED.raw_profile,
			updated_at = NOW()`,
		rec.KeycloakSub, rec.YandexID, rec.Login, rec.Email,
		rec.FirstName, rec.LastName, rec.DisplayName, rec.AvatarURL, rec.RawProfile,
	)
	return err
}

func (r *Repository) GrantConsent(ctx context.Context, keycloakSub string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE yandex_user_profiles
		SET consent_granted_at = NOW(), updated_at = NOW()
		WHERE keycloak_sub = $1`,
		keycloakSub,
	)
	return err
}
