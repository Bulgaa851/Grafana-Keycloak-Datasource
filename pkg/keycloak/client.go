// Package keycloak is a thin, server-side client for the Keycloak Admin REST
// API. It performs the OAuth2 client_credentials grant, caches the resulting
// access token, and exposes the handful of admin endpoints this datasource
// needs. All calls happen in the Grafana backend; the client secret never
// leaves this process and is never logged or placed into an error message.
package keycloak

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/example/keycloak-metrics-datasource/pkg/models"
)

// tokenExpiryBuffer is subtracted from the token lifetime so we refresh a little
// before the token actually expires, avoiding races against the expiry instant.
const tokenExpiryBuffer = 30 * time.Second

// Client talks to a single Keycloak realm using a confidential client.
type Client struct {
	baseURL      string
	realm        string
	clientID     string
	clientSecret string

	httpClient *http.Client

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

// New builds a Client from the datasource settings.
func New(s *models.PluginSettings) *Client {
	secret := ""
	if s.Secrets != nil {
		secret = s.Secrets.ClientSecret
	}
	return &Client{
		baseURL:      s.BaseURL,
		realm:        s.Realm,
		clientID:     s.ClientID,
		clientSecret: secret,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

// token returns a valid access token, fetching a new one via the
// client_credentials grant only when the cached token is missing or expired.
func (c *Client) token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)

	endpoint := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.baseURL, url.PathEscape(c.realm))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not reach Keycloak token endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var tr tokenResponse
	_ = json.Unmarshal(body, &tr)

	if resp.StatusCode != http.StatusOK || tr.AccessToken == "" {
		// Surface Keycloak's own error/description (e.g. "invalid_client").
		// These never contain the submitted secret value.
		msg := tr.Error
		if tr.ErrorDesc != "" {
			msg = fmt.Sprintf("%s: %s", tr.Error, tr.ErrorDesc)
		}
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return "", fmt.Errorf("authentication with Keycloak failed (%s)", msg)
	}

	c.accessToken = tr.AccessToken
	lifetime := time.Duration(tr.ExpiresIn) * time.Second
	if lifetime <= tokenExpiryBuffer {
		// Defensive: very short-lived token, keep it but don't go negative.
		c.tokenExpiry = time.Now().Add(lifetime)
	} else {
		c.tokenExpiry = time.Now().Add(lifetime - tokenExpiryBuffer)
	}

	return c.accessToken, nil
}

// invalidateToken clears the cached token so the next call re-authenticates.
func (c *Client) invalidateToken() {
	c.mu.Lock()
	c.accessToken = ""
	c.tokenExpiry = time.Time{}
	c.mu.Unlock()
}

// adminGet performs an authenticated GET against /admin/realms/{realm}{path}.
// It retries once after re-authenticating if the token was rejected (401).
func (c *Client) adminGet(ctx context.Context, path string, query url.Values) ([]byte, error) {
	body, status, err := c.doAdminGet(ctx, path, query)
	if err != nil {
		return nil, err
	}
	if status == http.StatusUnauthorized {
		// Cached token may have been revoked early; refresh once and retry.
		c.invalidateToken()
		body, status, err = c.doAdminGet(ctx, path, query)
		if err != nil {
			return nil, err
		}
	}
	if status != http.StatusOK {
		return nil, httpError(status, path, body)
	}
	return body, nil
}

func (c *Client) doAdminGet(ctx context.Context, path string, query url.Values) ([]byte, int, error) {
	tok, err := c.token(ctx)
	if err != nil {
		return nil, 0, err
	}

	endpoint := fmt.Sprintf("%s/admin/realms/%s%s", c.baseURL, url.PathEscape(c.realm), path)
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("could not reach Keycloak: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	return body, resp.StatusCode, nil
}

// httpError builds a readable, secret-free error from a non-200 admin response.
func httpError(status int, path string, body []byte) error {
	// Keycloak error bodies look like {"error":"..."} or {"errorMessage":"..."}.
	var ke struct {
		Error        string `json:"error"`
		ErrorMessage string `json:"errorMessage"`
	}
	_ = json.Unmarshal(body, &ke)
	detail := ke.ErrorMessage
	if detail == "" {
		detail = ke.Error
	}

	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		hint := "the service account is missing required realm-management roles (e.g. view-users, view-events, view-realm, query-users, query-groups)"
		if detail != "" {
			return fmt.Errorf("Keycloak denied the request for %s (HTTP %d: %s); %s", path, status, detail, hint)
		}
		return fmt.Errorf("Keycloak denied the request for %s (HTTP %d); %s", path, status, hint)
	case http.StatusNotFound:
		return fmt.Errorf("Keycloak resource not found for %s (HTTP 404); check the realm name and that the feature is enabled", path)
	default:
		if detail != "" {
			return fmt.Errorf("Keycloak request to %s failed (HTTP %d: %s)", path, status, detail)
		}
		return fmt.Errorf("Keycloak request to %s failed (HTTP %d)", path, status)
	}
}

// ----- Endpoint helpers -------------------------------------------------------

// UsersCount returns GET /admin/realms/{realm}/users/count (a bare integer).
func (c *Client) UsersCount(ctx context.Context) (int64, error) {
	body, err := c.adminGet(ctx, "/users/count", nil)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(body)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("unexpected users/count response from Keycloak")
	}
	return n, nil
}

// GroupsCount returns GET /admin/realms/{realm}/groups/count -> {"count": n}.
func (c *Client) GroupsCount(ctx context.Context) (int64, error) {
	body, err := c.adminGet(ctx, "/groups/count", nil)
	if err != nil {
		return 0, err
	}
	var r struct {
		Count int64 `json:"count"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return 0, fmt.Errorf("unexpected groups/count response from Keycloak")
	}
	return r.Count, nil
}

// Role is a realm role as returned by GET /admin/realms/{realm}/roles.
type Role struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Composite   bool   `json:"composite"`
	ClientRole  bool   `json:"clientRole"`
}

// RealmRoles returns GET /admin/realms/{realm}/roles.
func (c *Client) RealmRoles(ctx context.Context) ([]Role, error) {
	body, err := c.adminGet(ctx, "/roles", nil)
	if err != nil {
		return nil, err
	}
	var roles []Role
	if err := json.Unmarshal(body, &roles); err != nil {
		return nil, fmt.Errorf("unexpected roles response from Keycloak")
	}
	return roles, nil
}

// ActiveSessions returns the active session count for a client (by its clientId,
// not its internal UUID). It resolves the UUID first, then queries
// /clients/{uuid}/session-count.
func (c *Client) ActiveSessions(ctx context.Context, clientID string) (int64, error) {
	if strings.TrimSpace(clientID) == "" {
		return 0, fmt.Errorf("a Client ID is required for the active_sessions query")
	}

	q := url.Values{}
	q.Set("clientId", clientID)
	body, err := c.adminGet(ctx, "/clients", q)
	if err != nil {
		return 0, err
	}
	var clients []struct {
		ID       string `json:"id"`
		ClientID string `json:"clientId"`
	}
	if err := json.Unmarshal(body, &clients); err != nil {
		return 0, fmt.Errorf("unexpected clients response from Keycloak")
	}
	if len(clients) == 0 {
		return 0, fmt.Errorf("client %q was not found in realm %q", clientID, c.realm)
	}

	body, err = c.adminGet(ctx, "/clients/"+url.PathEscape(clients[0].ID)+"/session-count", nil)
	if err != nil {
		return 0, err
	}
	var r struct {
		Count int64 `json:"count"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return 0, fmt.Errorf("unexpected session-count response from Keycloak")
	}
	return r.Count, nil
}

// Event is a single entry from GET /admin/realms/{realm}/events. Time is epoch
// milliseconds.
type Event struct {
	Time     int64  `json:"time"`
	Type     string `json:"type"`
	ClientID string `json:"clientId"`
	UserID   string `json:"userId"`
}

// Events fetches realm events between fromMs and toMs (epoch ms), optionally
// filtered by event type. Keycloak's server-side date filter is day-grained, so
// we request the day range and then filter precisely by millisecond on our side.
func (c *Client) Events(ctx context.Context, eventType string, fromMs, toMs int64) ([]Event, error) {
	q := url.Values{}
	// dateFrom/dateTo are day-grained (yyyy-MM-dd). Keycloak compares against
	// midnight of each day, so it filters time >= midnight(dateFrom) and
	// time <= midnight(dateTo). To include the whole "to" day we pass the day
	// AFTER toMs; the precise millisecond trimming below keeps the result inside
	// the requested range.
	q.Set("dateFrom", time.UnixMilli(fromMs).UTC().Format("2006-01-02"))
	q.Set("dateTo", time.UnixMilli(toMs).UTC().AddDate(0, 0, 1).Format("2006-01-02"))
	q.Set("max", "10000")
	if strings.TrimSpace(eventType) != "" {
		q.Set("type", strings.TrimSpace(eventType))
	}

	body, err := c.adminGet(ctx, "/events", q)
	if err != nil {
		return nil, err
	}
	var events []Event
	if err := json.Unmarshal(body, &events); err != nil {
		return nil, fmt.Errorf("unexpected events response from Keycloak")
	}

	// Precise millisecond filtering to respect the dashboard time range.
	filtered := events[:0]
	for _, e := range events {
		if e.Time >= fromMs && e.Time <= toMs {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}
