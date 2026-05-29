package yandex

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const userInfoURL = "https://login.yandex.ru/info?format=json"
const authorizeURL = "https://oauth.yandex.ru/authorize"
const tokenURL = "https://oauth.yandex.ru/token"

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

func (c *Client) BuildAuthURL(clientID, redirectURI, state, codeChallenge string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")
	return authorizeURL + "?" + q.Encode()
}

func (c *Client) ExchangeCode(clientID, clientSecret, redirectURI, code, codeVerifier string) (string, int, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("redirect_uri", redirectURI)
	form.Set("code_verifier", codeVerifier)

	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("yandex token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var payload struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", 0, err
	}
	if payload.AccessToken == "" {
		return "", 0, fmt.Errorf("yandex token exchange returned empty access token")
	}
	return payload.AccessToken, payload.ExpiresIn, nil
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
