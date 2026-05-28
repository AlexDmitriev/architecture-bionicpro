package keycloak

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) adminToken() (string, error) {
	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", "admin-cli")
	form.Set("username", c.cfg.KeycloakAdminUser)
	form.Set("password", c.cfg.KeycloakAdminPassword)

	endpoint := fmt.Sprintf(
		"%s/realms/master/protocol/openid-connect/token",
		strings.TrimRight(c.cfg.KeycloakURL, "/"),
	)
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("admin token failed (%d): %s", resp.StatusCode, string(body))
	}

	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.AccessToken == "" {
		return "", fmt.Errorf("admin token is empty")
	}
	return payload.AccessToken, nil
}

func (c *Client) EnsureUser(email, username, firstName, lastName string) (string, string, error) {
	adminToken, err := c.adminToken()
	if err != nil {
		return "", "", err
	}

	usersEndpoint := fmt.Sprintf(
		"%s/admin/realms/%s/users",
		strings.TrimRight(c.cfg.KeycloakURL, "/"),
		c.cfg.KeycloakRealm,
	)

	query := url.Values{}
	query.Set("email", email)
	query.Set("exact", "true")
	req, err := http.NewRequest(http.MethodGet, usersEndpoint+"?"+query.Encode(), nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("user search failed (%d): %s", resp.StatusCode, string(body))
	}
	var users []struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	if err := json.Unmarshal(body, &users); err != nil {
		return "", "", err
	}
	if len(users) > 0 && users[0].ID != "" {
		return users[0].ID, users[0].Username, nil
	}

	payload := map[string]interface{}{
		"username":      username,
		"email":         email,
		"firstName":     firstName,
		"lastName":      lastName,
		"enabled":       true,
		"emailVerified": true,
	}
	raw, _ := json.Marshal(payload)
	createReq, err := http.NewRequest(http.MethodPost, usersEndpoint, bytes.NewReader(raw))
	if err != nil {
		return "", "", err
	}
	createReq.Header.Set("Authorization", "Bearer "+adminToken)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := c.httpClient.Do(createReq)
	if err != nil {
		return "", "", err
	}
	defer createResp.Body.Close()
	createBody, _ := io.ReadAll(createResp.Body)
	if createResp.StatusCode != http.StatusCreated {
		return "", "", fmt.Errorf("create user failed (%d): %s", createResp.StatusCode, string(createBody))
	}

	// Re-read created user id.
	readReq, err := http.NewRequest(http.MethodGet, usersEndpoint+"?"+query.Encode(), nil)
	if err != nil {
		return "", "", err
	}
	readReq.Header.Set("Authorization", "Bearer "+adminToken)
	readResp, err := c.httpClient.Do(readReq)
	if err != nil {
		return "", "", err
	}
	defer readResp.Body.Close()
	readBody, _ := io.ReadAll(readResp.Body)
	if readResp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("re-read user failed (%d): %s", readResp.StatusCode, string(readBody))
	}
	var created []struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	if err := json.Unmarshal(readBody, &created); err != nil {
		return "", "", err
	}
	if len(created) == 0 || created[0].ID == "" {
		return "", "", fmt.Errorf("created user id not found")
	}
	return created[0].ID, created[0].Username, nil
}

func (c *Client) SetUserPassword(userID, password string) error {
	adminToken, err := c.adminToken()
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf(
		"%s/admin/realms/%s/users/%s/reset-password",
		strings.TrimRight(c.cfg.KeycloakURL, "/"),
		c.cfg.KeycloakRealm,
		userID,
	)
	payload := map[string]interface{}{
		"type":      "password",
		"value":     password,
		"temporary": false,
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPut, endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("set password failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func GenerateServicePassword() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
