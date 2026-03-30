package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultBaseURL = "https://api.flagify.dev"

type Client struct {
	baseURL        string
	httpClient     *http.Client
	token          string
	refreshToken   string
	OnTokenRefresh func(accessToken, refreshToken string)
}

func NewClient(token string) *Client {
	return &Client{
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		token: token,
	}
}

func (c *Client) SetRefreshToken(token string) {
	c.refreshToken = token
}

func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}

func (c *Client) SetToken(token string) {
	c.token = token
}

func (c *Client) Get(path string, result any) error {
	return c.do("GET", path, nil, result)
}

func (c *Client) Post(path string, body, result any) error {
	return c.do("POST", path, body, result)
}

func (c *Client) Patch(path string, body, result any) error {
	return c.do("PATCH", path, body, result)
}

func (c *Client) Put(path string, body, result any) error {
	return c.do("PUT", path, body, result)
}

func (c *Client) Delete(path string) error {
	return c.do("DELETE", path, nil, nil)
}

func (c *Client) do(method, path string, body, result any) error {
	err := c.doOnce(method, path, body, result)
	if err == nil {
		return nil
	}

	// If 401 and we have a refresh token, try to refresh and retry once
	if isUnauthorized(err) && c.refreshToken != "" && c.OnTokenRefresh != nil {
		tokens, refreshErr := c.Refresh(c.refreshToken)
		if refreshErr != nil {
			return err // return original error
		}
		c.token = tokens.AccessToken
		c.refreshToken = tokens.RefreshToken
		c.OnTokenRefresh(tokens.AccessToken, tokens.RefreshToken)
		return c.doOnce(method, path, body, result)
	}

	return err
}

func (c *Client) doOnce(method, path string, body, result any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var apiErr struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
			return fmt.Errorf("API error %d", resp.StatusCode)
		}
		return &APIError{StatusCode: resp.StatusCode, Code: apiErr.Code, Message: apiErr.Message}
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

// APIError represents a structured error from the API.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func isUnauthorized(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == http.StatusUnauthorized
	}
	return false
}

// Auth

type AuthResponse struct {
	User   map[string]any `json:"user"`
	Tokens TokenPair      `json:"tokens"`
}

type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

func (c *Client) Login(email, password, deviceID string) (*AuthResponse, error) {
	var result AuthResponse
	err := c.Post("/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
		"deviceId": deviceID,
	}, &result)
	return &result, err
}

func (c *Client) Register(email, password, name, deviceID string) (*AuthResponse, error) {
	var result AuthResponse
	err := c.Post("/v1/auth/register", map[string]string{
		"email":    email,
		"password": password,
		"name":     name,
		"deviceId": deviceID,
	}, &result)
	return &result, err
}

func (c *Client) Refresh(refreshToken string) (*TokenPair, error) {
	var result TokenPair
	err := c.doOnce("POST", "/v1/auth/refresh", map[string]string{
		"refreshToken": refreshToken,
	}, &result)
	return &result, err
}

// Workspaces

type Workspace struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Plan     string `json:"plan"`
}

func (c *Client) ListWorkspaces() ([]Workspace, error) {
	var result []Workspace
	err := c.Get("/v1/workspaces", &result)
	return result, err
}

// Projects

type Project struct {
	ID           string        `json:"id"`
	WorkspaceID  string        `json:"workspaceId"`
	Name         string        `json:"name"`
	Slug         string        `json:"slug"`
	Environments []Environment `json:"environments,omitempty"`
}

type Environment struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

func (c *Client) ListProjects(workspaceID string) ([]Project, error) {
	var result []Project
	err := c.Get("/v1/workspaces/"+workspaceID+"/projects", &result)
	return result, err
}

func (c *Client) GetProject(projectID string) (*Project, error) {
	var result Project
	err := c.Get("/v1/projects/"+projectID, &result)
	return &result, err
}

// Flags

type Flag struct {
	ID           string          `json:"id"`
	Key          string          `json:"key"`
	Name         string          `json:"name"`
	Type         string          `json:"type"`
	DefaultValue json.RawMessage `json:"defaultValue"`
	Environments []FlagEnv       `json:"environments,omitempty"`
}

type FlagEnv struct {
	ID                string `json:"id"`
	EnvironmentID     string `json:"environmentId"`
	EnvironmentKey    string `json:"environmentKey"`
	Enabled           bool   `json:"enabled"`
	RolloutPercentage *int   `json:"rolloutPercentage,omitempty"`
}

func (c *Client) ListFlags(projectID string) ([]Flag, error) {
	var result []Flag
	err := c.Get("/v1/projects/"+projectID+"/flags", &result)
	return result, err
}

func (c *Client) CreateFlag(projectID string, body map[string]any) (*Flag, error) {
	var result Flag
	err := c.Post("/v1/projects/"+projectID+"/flags", body, &result)
	return &result, err
}

func (c *Client) GetFlag(flagID string) (*Flag, error) {
	var result Flag
	err := c.Get("/v1/flags/"+flagID, &result)
	return &result, err
}

func (c *Client) ToggleFlag(flagEnvID string, enabled bool) error {
	return c.Put("/v1/flag-environments/"+flagEnvID, map[string]any{
		"enabled": enabled,
	}, nil)
}

// API Keys

type APIKey struct {
	ID            string     `json:"id"`
	EnvironmentID string     `json:"environmentId"`
	Type          string     `json:"type"`
	Prefix        string     `json:"prefix"`
	LastUsedAt    *time.Time `json:"lastUsedAt,omitempty"`
	RevokedAt     *time.Time `json:"revokedAt,omitempty"`
	CreatedBy     string     `json:"createdBy"`
	CreatedAt     time.Time  `json:"createdAt"`
}

type KeyPairResponse struct {
	PublishableKey string `json:"publishableKey"`
	SecretKey      string `json:"secretKey"`
	Publishable    APIKey `json:"publishable"`
	Secret         APIKey `json:"secret"`
}

func (c *Client) GenerateKeys(environmentID string) (*KeyPairResponse, error) {
	var result KeyPairResponse
	err := c.Post("/v1/environments/"+environmentID+"/keys", nil, &result)
	return &result, err
}

func (c *Client) ListKeys(environmentID string) ([]APIKey, error) {
	var result []APIKey
	err := c.Get("/v1/environments/"+environmentID+"/keys", &result)
	return result, err
}

func (c *Client) RevokeKeys(environmentID string) error {
	return c.Post("/v1/environments/"+environmentID+"/keys/revoke", nil, nil)
}
