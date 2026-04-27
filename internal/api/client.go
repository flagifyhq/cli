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
	// Tags every CLI-originated audit event with `source: "cli"` so operators
	// can tell them apart from MCP (`mcp`) or console (`web`) entries.
	req.Header.Set("X-Flagify-Source", "cli")
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

// Users

type UserMe struct {
	ID    string  `json:"id"`
	Email string  `json:"email"`
	Name  *string `json:"name,omitempty"`
}

func (c *Client) GetMe() (*UserMe, error) {
	var result UserMe
	err := c.Get("/v1/users/me", &result)
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

func (c *Client) DeleteProject(projectID string) error {
	return c.Delete("/v1/projects/" + projectID)
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
	ID                string        `json:"id"`
	EnvironmentID     string        `json:"environmentId"`
	EnvironmentKey    string        `json:"environmentKey"`
	Enabled           bool          `json:"enabled"`
	RolloutPercentage *int          `json:"rolloutPercentage,omitempty"`
	Variants          []FlagVariant `json:"variants,omitempty"`
}

type FlagVariant struct {
	Key    string `json:"key"`
	Value  any    `json:"value"`
	Weight int    `json:"weight"`
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

// ToggleFlagByKey hits the project-scoped route that accepts the flag key and
// environment slug directly, so the caller does not need to look up the
// flag_environments ULID first.
func (c *Client) ToggleFlagByKey(projectID, flagKey, envKey string, enabled bool) error {
	return c.Put("/v1/projects/"+projectID+"/flags/"+flagKey+"/environments/"+envKey, map[string]any{
		"enabled": enabled,
	}, nil)
}


// Segments

type Segment struct {
	ID        string        `json:"id"`
	ProjectID string        `json:"projectId"`
	Name      string        `json:"name"`
	MatchType string        `json:"matchType"`
	Rules     []SegmentRule `json:"rules,omitempty"`
}

type SegmentRule struct {
	Attribute string `json:"attribute"`
	Operator  string `json:"operator"`
	Value     any    `json:"value"`
}

func (c *Client) ListSegments(projectID string) ([]Segment, error) {
	var result []Segment
	err := c.Get("/v1/projects/"+projectID+"/segments", &result)
	return result, err
}

func (c *Client) CreateSegment(projectID string, body map[string]any) (*Segment, error) {
	var result Segment
	err := c.Post("/v1/projects/"+projectID+"/segments", body, &result)
	return &result, err
}

func (c *Client) DeleteSegment(segmentID string) error {
	return c.Delete("/v1/segments/" + segmentID)
}

// Targeting

type TargetingRule struct {
	ID                string               `json:"id"`
	Priority          int                  `json:"priority"`
	SegmentID         *string              `json:"segmentId,omitempty"`
	ValueOverride     any                  `json:"valueOverride,omitempty"`
	RolloutPercentage *int                 `json:"rolloutPercentage,omitempty"`
	Enabled           bool                 `json:"enabled"`
	Conditions        []TargetingCondition `json:"conditions,omitempty"`
}

type TargetingCondition struct {
	Attribute string `json:"attribute"`
	Operator  string `json:"operator"`
	Value     any    `json:"value"`
}

func (c *Client) GetTargetingRules(flagEnvID string) ([]TargetingRule, error) {
	var result []TargetingRule
	err := c.Get("/v1/flag-environments/"+flagEnvID+"/targeting-rules", &result)
	return result, err
}

func (c *Client) SetTargetingRules(flagEnvID string, body map[string]any) ([]TargetingRule, error) {
	var result []TargetingRule
	err := c.Put("/v1/flag-environments/"+flagEnvID+"/targeting-rules", body, &result)
	return result, err
}

// GetTargetingRulesByKey is the slug-friendly variant of GetTargetingRules.
func (c *Client) GetTargetingRulesByKey(projectID, flagKey, envKey string) ([]TargetingRule, error) {
	var result []TargetingRule
	err := c.Get("/v1/projects/"+projectID+"/flags/"+flagKey+"/environments/"+envKey+"/targeting-rules", &result)
	return result, err
}

// SetTargetingRulesByKey is the slug-friendly variant of SetTargetingRules.
func (c *Client) SetTargetingRulesByKey(projectID, flagKey, envKey string, body map[string]any) ([]TargetingRule, error) {
	var result []TargetingRule
	err := c.Put("/v1/projects/"+projectID+"/flags/"+flagKey+"/environments/"+envKey+"/targeting-rules", body, &result)
	return result, err
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

// GenerateKeysByEnv hits the project-scoped route that accepts the environment
// slug directly, so the caller does not need to resolve it to a ULID first.
func (c *Client) GenerateKeysByEnv(projectID, envKey string) (*KeyPairResponse, error) {
	var result KeyPairResponse
	err := c.Post("/v1/projects/"+projectID+"/environments/"+envKey+"/keys", nil, &result)
	return &result, err
}

// ListKeysByEnv is the slug-friendly variant of ListKeys.
func (c *Client) ListKeysByEnv(projectID, envKey string) ([]APIKey, error) {
	var result []APIKey
	err := c.Get("/v1/projects/"+projectID+"/environments/"+envKey+"/keys", &result)
	return result, err
}

// RevokeKeysByEnv is the slug-friendly variant of RevokeKeys.
func (c *Client) RevokeKeysByEnv(projectID, envKey string) error {
	return c.Post("/v1/projects/"+projectID+"/environments/"+envKey+"/keys/revoke", nil, nil)
}

// RevokeKeyByID revokes a single API key by its ID.
func (c *Client) RevokeKeyByID(environmentID, keyID string) error {
	return c.Post("/v1/environments/"+environmentID+"/keys/"+keyID+"/revoke", nil, nil)
}

// RevokeKeyByEnv is the slug-friendly variant of RevokeKeyByID.
func (c *Client) RevokeKeyByEnv(projectID, envKey, keyID string) error {
	return c.Post("/v1/projects/"+projectID+"/environments/"+envKey+"/keys/"+keyID+"/revoke", nil, nil)
}

type HealthIssue struct {
	FlagID      string `json:"flagId"`
	FlagKey     string `json:"flagKey"`
	FlagName    string `json:"flagName"`
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Message     string `json:"message"`
	Environment string `json:"environment,omitempty"`
	RuleID      string `json:"ruleId,omitempty"`
	Fix         string `json:"fix,omitempty"`
}

func (c *Client) GetFlagHealth(projectID string) ([]HealthIssue, error) {
	var result []HealthIssue
	err := c.Get("/v1/projects/"+projectID+"/overview/health", &result)
	return result, err
}

// Webhooks

type Webhook struct {
	ID            string     `json:"id"`
	ProjectID     string     `json:"projectId"`
	EnvironmentID string     `json:"environmentId"`
	Name          string     `json:"name"`
	URL           string     `json:"url"`
	// Returned only on Create — subsequent reads omit the field.
	Secret     string     `json:"secret,omitempty"`
	Events     []string   `json:"events"`
	Active     bool       `json:"active"`
	DisabledAt *time.Time `json:"disabledAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

type WebhookDelivery struct {
	ID           string     `json:"id"`
	WebhookID    string     `json:"webhookId"`
	EventAction  string     `json:"eventAction"`
	Status       string     `json:"status"`
	Attempt      int        `json:"attempt"`
	ResponseCode *int       `json:"responseCode,omitempty"`
	Error        *string    `json:"error,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	DeliveredAt  *time.Time `json:"deliveredAt,omitempty"`
}

// webhookDeliveriesPage matches the API's paginated response shape. The
// CLI flattens it to a slice for table rendering and ignores the cursor
// because the command surfaces only the first page.
type webhookDeliveriesPage struct {
	Data       []WebhookDelivery `json:"data"`
	HasMore    bool              `json:"hasMore"`
	NextCursor string            `json:"nextCursor,omitempty"`
}

// ListWebhooks lists webhooks for a project. When `envKeyOrID` is set the
// API restricts the result to the environment-scoped subset; an empty
// value falls back to the project-aggregate view.
func (c *Client) ListWebhooks(projectID, envKeyOrID string) ([]Webhook, error) {
	var result []Webhook
	path := "/v1/projects/" + projectID + "/webhooks"
	if envKeyOrID != "" {
		path = "/v1/projects/" + projectID + "/environments/" + envKeyOrID + "/webhooks"
	}
	err := c.Get(path, &result)
	return result, err
}

// CreateWebhook always targets a specific environment; the env identifier
// (slug or ULID) is required and is sent as a path segment so the API's
// scoped middleware resolves it before the handler runs.
func (c *Client) CreateWebhook(projectID, envKeyOrID string, body map[string]any) (*Webhook, error) {
	var result Webhook
	err := c.Post("/v1/projects/"+projectID+"/environments/"+envKeyOrID+"/webhooks", body, &result)
	return &result, err
}

func (c *Client) GetWebhook(projectID, webhookID string) (*Webhook, error) {
	var result Webhook
	err := c.Get("/v1/projects/"+projectID+"/webhooks/"+webhookID, &result)
	return &result, err
}

func (c *Client) DeleteWebhook(projectID, webhookID string) error {
	return c.Delete("/v1/projects/" + projectID + "/webhooks/" + webhookID)
}

func (c *Client) ListWebhookDeliveries(projectID, webhookID string) ([]WebhookDelivery, error) {
	var page webhookDeliveriesPage
	err := c.Get("/v1/projects/"+projectID+"/webhooks/"+webhookID+"/deliveries", &page)
	return page.Data, err
}
