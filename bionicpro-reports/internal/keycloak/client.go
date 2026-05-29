package keycloak

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"bionicpro-reports/internal/config"
)

type Client struct {
	cfg        config.Config
	httpClient *http.Client
}

type UserInfo struct {
	Sub               string `json:"sub"`
	Email             string `json:"email"`
	PreferredUsername string `json:"preferred_username"`
}

func NewClient(cfg config.Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) userInfoEndpoint() string {
	return fmt.Sprintf(
		"%s/realms/%s/protocol/openid-connect/userinfo",
		strings.TrimRight(c.cfg.KeycloakURL, "/"),
		c.cfg.KeycloakRealm,
	)
}

func (c *Client) FetchUserInfo(accessToken string) (*UserInfo, error) {
	req, err := http.NewRequest(http.MethodGet, c.userInfoEndpoint(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo failed (%d): %s", resp.StatusCode, string(body))
	}

	var ui UserInfo
	if err := json.Unmarshal(body, &ui); err != nil {
		return nil, err
	}
	return &ui, nil
}
