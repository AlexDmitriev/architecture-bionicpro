package yandex

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const userInfoURL = "https://login.yandex.ru/info?format=json"

type Profile struct {
	ID              string `json:"id"`
	Login           string `json:"login"`
	DefaultEmail    string `json:"default_email"`
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	DisplayName     string `json:"display_name"`
	DefaultAvatarID string `json:"default_avatar_id"`
}

func (p Profile) AvatarURL() string {
	if p.DefaultAvatarID == "" {
		return ""
	}
	return fmt.Sprintf("https://avatars.yandex.net/get-yapic/%s/islands-200", p.DefaultAvatarID)
}

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{httpClient: &http.Client{Timeout: 15 * time.Second}}
}

func (c *Client) FetchProfile(accessToken string) (*Profile, json.RawMessage, error) {
	req, err := http.NewRequest(http.MethodGet, userInfoURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "OAuth "+strings.TrimSpace(accessToken))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("yandex userinfo failed (%d): %s", resp.StatusCode, string(body))
	}

	var p Profile
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, nil, err
	}
	return &p, json.RawMessage(body), nil
}
