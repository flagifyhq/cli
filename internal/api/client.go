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
	baseURL    string
	httpClient *http.Client
	token      string
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
		return fmt.Errorf("%s: %s", apiErr.Code, apiErr.Message)
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
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
	err := c.Post("/v1/auth/refresh", map[string]string{
		"refreshToken": refreshToken,
	}, &result)
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
