package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const sessionKeyPrefix = "session:"

type OAuthState struct {
	Verifier string `json:"verifier"`
	IdpHint  string `json:"idp_hint,omitempty"`
}

type Data struct {
	AccessToken           string    `json:"access_token"`
	RefreshTokenEncrypted string    `json:"refresh_token_enc"`
	AccessExpiresAt       time.Time `json:"access_expires_at"`
	RefreshExpiresAt      time.Time `json:"refresh_expires_at,omitempty"`
	KeycloakSub           string    `json:"keycloak_sub,omitempty"`
	IdentityProvider      string    `json:"identity_provider,omitempty"`
}

type Store struct {
	rdb    *redis.Client
	encKey []byte
	ttl    time.Duration
}

func NewStore(rdb *redis.Client, encKey []byte, ttl time.Duration) *Store {
	return &Store{rdb: rdb, encKey: encKey, ttl: ttl}
}

func (s *Store) key(id string) string {
	return sessionKeyPrefix + id
}

func (s *Store) Save(ctx context.Context, id string, data Data, refreshToken string) error {
	encRefresh, err := Encrypt(s.encKey, refreshToken)
	if err != nil {
		return fmt.Errorf("encrypt refresh token: %w", err)
	}
	data.RefreshTokenEncrypted = encRefresh
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, s.key(id), raw, s.ttl).Err()
}

func (s *Store) Get(ctx context.Context, id string) (*Data, error) {
	raw, err := s.rdb.Get(ctx, s.key(id)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var data Data
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (s *Store) GetRefreshToken(ctx context.Context, id string) (string, error) {
	data, err := s.Get(ctx, id)
	if err != nil || data == nil {
		return "", err
	}
	return Decrypt(s.encKey, data.RefreshTokenEncrypted)
}

func (s *Store) Delete(ctx context.Context, id string) error {
	return s.rdb.Del(ctx, s.key(id)).Err()
}

func (s *Store) Rotate(ctx context.Context, oldID, newID string) error {
	data, err := s.Get(ctx, oldID)
	if err != nil || data == nil {
		return fmt.Errorf("session not found")
	}
	refresh, err := Decrypt(s.encKey, data.RefreshTokenEncrypted)
	if err != nil {
		return err
	}
	if err := s.Save(ctx, newID, *data, refresh); err != nil {
		return err
	}
	return s.Delete(ctx, oldID)
}

func (s *Store) SaveOAuthState(ctx context.Context, state string, oauth OAuthState) error {
	raw, err := json.Marshal(oauth)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, "oauth:state:"+state, raw, 10*time.Minute).Err()
}

func (s *Store) GetOAuthState(ctx context.Context, state string) (*OAuthState, error) {
	raw, err := s.rdb.Get(ctx, "oauth:state:"+state).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var oauth OAuthState
	if err := json.Unmarshal(raw, &oauth); err != nil {
		return nil, err
	}
	return &oauth, nil
}

func (s *Store) DeleteOAuthState(ctx context.Context, state string) error {
	return s.rdb.Del(ctx, "oauth:state:"+state).Err()
}
