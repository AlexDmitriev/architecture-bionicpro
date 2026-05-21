package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const sessionKeyPrefix = "session:"

type Data struct {
	AccessToken              string    `json:"access_token"`
	RefreshTokenEncrypted    string    `json:"refresh_token_enc"`
	AccessExpiresAt          time.Time `json:"access_expires_at"`
	RefreshExpiresAt         time.Time `json:"refresh_expires_at,omitempty"`
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

func (s *Store) Save(ctx context.Context, id string, accessToken, refreshToken string, accessExpiresAt, refreshExpiresAt time.Time) error {
	encRefresh, err := Encrypt(s.encKey, refreshToken)
	if err != nil {
		return fmt.Errorf("encrypt refresh token: %w", err)
	}
	data := Data{
		AccessToken:           accessToken,
		RefreshTokenEncrypted: encRefresh,
		AccessExpiresAt:       accessExpiresAt,
		RefreshExpiresAt:      refreshExpiresAt,
	}
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
	if err := s.Save(ctx, newID, data.AccessToken, refresh, data.AccessExpiresAt, data.RefreshExpiresAt); err != nil {
		return err
	}
	return s.Delete(ctx, oldID)
}

func (s *Store) SavePKCE(ctx context.Context, state, verifier string) error {
	return s.rdb.Set(ctx, "oauth:state:"+state, verifier, 10*time.Minute).Err()
}

func (s *Store) GetPKCE(ctx context.Context, state string) (string, error) {
	v, err := s.rdb.Get(ctx, "oauth:state:"+state).Result()
	if err == redis.Nil {
		return "", nil
	}
	return v, err
}

func (s *Store) DeletePKCE(ctx context.Context, state string) error {
	return s.rdb.Del(ctx, "oauth:state:"+state).Err()
}
