package keycloak

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"bionicpro-auth/internal/config"
)

type Client struct {
	cfg        config.Config
	httpClient *http.Client
}

func NewClient(cfg config.Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	TokenType        string `json:"token_type"`
}

func (c *Client) tokenEndpoint() string {
	return fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token",
		strings.TrimRight(c.cfg.KeycloakURL, "/"), c.cfg.KeycloakRealm)
}

func (c *Client) authEndpoint() string {
	return fmt.Sprintf("%s/realms/%s/protocol/openid-connect/auth",
		strings.TrimRight(c.cfg.KeycloakURL, "/"), c.cfg.KeycloakRealm)
}

func (c *Client) logoutEndpoint() string {
	return fmt.Sprintf("%s/realms/%s/protocol/openid-connect/logout",
		strings.TrimRight(c.cfg.KeycloakURL, "/"), c.cfg.KeycloakRealm)
}

func (c *Client) BuildAuthURL(state, codeChallenge string) string {
	q := url.Values{}
	q.Set("client_id", c.cfg.KeycloakClientID)
	q.Set("redirect_uri", c.cfg.RedirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "openid")
	q.Set("state", state)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")
	return c.authEndpoint() + "?" + q.Encode()
}

func (c *Client) ExchangeCode(code, codeVerifier string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", c.cfg.KeycloakClientID)
	data.Set("client_secret", c.cfg.KeycloakSecret)
	data.Set("code", code)
	data.Set("redirect_uri", c.cfg.RedirectURI)
	data.Set("code_verifier", codeVerifier)
	return c.postToken(data)
}

func (c *Client) RefreshToken(refreshToken string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", c.cfg.KeycloakClientID)
	data.Set("client_secret", c.cfg.KeycloakSecret)
	data.Set("refresh_token", refreshToken)
	return c.postToken(data)
}

func (c *Client) postToken(data url.Values) (*TokenResponse, error) {
	req, err := http.NewRequest(http.MethodPost, c.tokenEndpoint(), strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed (%d): %s", resp.StatusCode, string(body))
	}

	var tr TokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, err
	}
	return &tr, nil
}

func (c *Client) Logout(refreshToken string) error {
	data := url.Values{}
	data.Set("client_id", c.cfg.KeycloakClientID)
	data.Set("client_secret", c.cfg.KeycloakSecret)
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequest(http.MethodPost, c.logoutEndpoint(), strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
