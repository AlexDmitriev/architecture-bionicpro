package keycloak

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type UserInfo struct {
	Sub                string `json:"sub"`
	Email              string `json:"email"`
	PreferredUsername  string `json:"preferred_username"`
	GivenName          string `json:"given_name"`
	FamilyName         string `json:"family_name"`
	Name               string `json:"name"`
	IdentityProvider   string `json:"identity_provider"`
}

func (c *Client) userInfoEndpoint() string {
	return fmt.Sprintf("%s/realms/%s/protocol/openid-connect/userinfo",
		strings.TrimRight(c.cfg.KeycloakURL, "/"), c.cfg.KeycloakRealm)
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

func (c *Client) ExchangeForIdpToken(subjectToken, requestedIssuer string) (string, error) {
	data := url.Values{}
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	data.Set("client_id", c.cfg.KeycloakClientID)
	data.Set("client_secret", c.cfg.KeycloakSecret)
	data.Set("subject_token", subjectToken)
	data.Set("subject_token_type", "urn:ietf:params:oauth:token-type:access_token")
	data.Set("requested_token_type", "urn:ietf:params:oauth:token-type:access_token")
	data.Set("requested_issuer", requestedIssuer)

	tr, err := c.postToken(data)
	if err != nil {
		return "", err
	}
	return tr.AccessToken, nil
}
