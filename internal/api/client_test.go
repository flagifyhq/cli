package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flagifyhq/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/test", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := api.NewClient("test-token")
	client.SetBaseURL(server.URL)

	var result map[string]string
	err := client.Get("/v1/test", &result)
	require.NoError(t, err)
	assert.Equal(t, "ok", result["status"])
}

func TestClientPost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "hello", body["key"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": "123"})
	}))
	defer server.Close()

	client := api.NewClient("token")
	client.SetBaseURL(server.URL)

	var result map[string]string
	err := client.Post("/v1/create", map[string]string{"key": "hello"}, &result)
	require.NoError(t, err)
	assert.Equal(t, "123", result["id"])
}

func TestClientErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"code":    "unauthorized",
			"message": "invalid token",
		})
	}))
	defer server.Close()

	client := api.NewClient("bad-token")
	client.SetBaseURL(server.URL)

	var result map[string]string
	err := client.Get("/v1/test", &result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
	assert.Contains(t, err.Error(), "invalid token")
}

func TestClientLogin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/auth/login", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.AuthResponse{
			User: map[string]any{"email": "test@example.com"},
			Tokens: api.TokenPair{
				AccessToken:  "access-123",
				RefreshToken: "refresh-456",
			},
		})
	}))
	defer server.Close()

	client := api.NewClient("")
	client.SetBaseURL(server.URL)

	result, err := client.Login("test@example.com", "password", "cli-test")
	require.NoError(t, err)
	assert.Equal(t, "access-123", result.Tokens.AccessToken)
	assert.Equal(t, "refresh-456", result.Tokens.RefreshToken)
}

func TestClientAutoRefreshOn401(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/auth/refresh" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(api.TokenPair{
				AccessToken:  "new-access",
				RefreshToken: "new-refresh",
			})
			return
		}

		callCount++
		if callCount == 1 {
			// First call: return 401
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"code":    "unauthorized",
				"message": "token expired",
			})
			return
		}
		// Second call (after refresh): succeed
		assert.Equal(t, "Bearer new-access", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	var savedAccess, savedRefresh string
	client := api.NewClient("expired-token")
	client.SetBaseURL(server.URL)
	client.SetRefreshToken("valid-refresh")
	client.OnTokenRefresh = func(access, refresh string) {
		savedAccess = access
		savedRefresh = refresh
	}

	var result map[string]string
	err := client.Get("/v1/test", &result)
	require.NoError(t, err)
	assert.Equal(t, "ok", result["status"])
	assert.Equal(t, "new-access", savedAccess)
	assert.Equal(t, "new-refresh", savedRefresh)
}

func TestClientNoRefreshOnAuthEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"code":    "unauthorized",
			"message": "invalid credentials",
		})
	}))
	defer server.Close()

	refreshCalled := false
	client := api.NewClient("bad-token")
	client.SetBaseURL(server.URL)
	// No refresh token set — should not attempt refresh
	client.OnTokenRefresh = func(access, refresh string) {
		refreshCalled = true
	}

	var result map[string]string
	err := client.Get("/v1/test", &result)
	require.Error(t, err)
	assert.False(t, refreshCalled)
}

func TestClientRefreshFailsFallsBack(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"code":    "unauthorized",
			"message": "token expired",
		})
	}))
	defer server.Close()

	client := api.NewClient("expired-token")
	client.SetBaseURL(server.URL)
	client.SetRefreshToken("also-expired")
	client.OnTokenRefresh = func(access, refresh string) {}

	var result map[string]string
	err := client.Get("/v1/test", &result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestClientListWorkspaces(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/workspaces", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]api.Workspace{
			{ID: "ws1", Name: "Acme Corp", Slug: "acme-corp", Plan: "pro"},
			{ID: "ws2", Name: "Side Project", Slug: "side-project", Plan: "free"},
		})
	}))
	defer server.Close()

	client := api.NewClient("token")
	client.SetBaseURL(server.URL)

	workspaces, err := client.ListWorkspaces()
	require.NoError(t, err)
	assert.Len(t, workspaces, 2)
	assert.Equal(t, "Acme Corp", workspaces[0].Name)
	assert.Equal(t, "pro", workspaces[0].Plan)
}

func TestClientListProjects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/workspaces/ws1/projects", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]api.Project{
			{ID: "p1", Name: "Web App", Slug: "web-app"},
			{ID: "p2", Name: "Mobile App", Slug: "mobile-app"},
		})
	}))
	defer server.Close()

	client := api.NewClient("token")
	client.SetBaseURL(server.URL)

	projects, err := client.ListProjects("ws1")
	require.NoError(t, err)
	assert.Len(t, projects, 2)
	assert.Equal(t, "Web App", projects[0].Name)
}

func TestClientGetProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/projects/p1", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.Project{
			ID:   "p1",
			Name: "Web App",
			Slug: "web-app",
			Environments: []api.Environment{
				{ID: "e1", Key: "development", Name: "Development"},
				{ID: "e2", Key: "staging", Name: "Staging"},
				{ID: "e3", Key: "production", Name: "Production"},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient("token")
	client.SetBaseURL(server.URL)

	project, err := client.GetProject("p1")
	require.NoError(t, err)
	assert.Equal(t, "Web App", project.Name)
	assert.Len(t, project.Environments, 3)
	assert.Equal(t, "development", project.Environments[0].Key)
}

func TestClientGenerateKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/environments/env1/keys", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(api.KeyPairResponse{
			PublishableKey: "pk_dev_abc123",
			SecretKey:      "sk_dev_abc123",
			Publishable: api.APIKey{
				ID:     "key1",
				Type:   "publishable",
				Prefix: "pk_dev_abc",
			},
			Secret: api.APIKey{
				ID:     "key2",
				Type:   "secret",
				Prefix: "sk_dev_abc",
			},
		})
	}))
	defer server.Close()

	client := api.NewClient("token")
	client.SetBaseURL(server.URL)

	result, err := client.GenerateKeys("env1")
	require.NoError(t, err)
	assert.Equal(t, "pk_dev_abc123", result.PublishableKey)
	assert.Equal(t, "sk_dev_abc123", result.SecretKey)
	assert.Equal(t, "publishable", result.Publishable.Type)
	assert.Equal(t, "secret", result.Secret.Type)
}

func TestClientListKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/environments/env1/keys", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]api.APIKey{
			{ID: "key1", Type: "publishable", Prefix: "pk_dev_abc"},
			{ID: "key2", Type: "secret", Prefix: "sk_dev_abc"},
		})
	}))
	defer server.Close()

	client := api.NewClient("token")
	client.SetBaseURL(server.URL)

	keys, err := client.ListKeys("env1")
	require.NoError(t, err)
	assert.Len(t, keys, 2)
	assert.Equal(t, "publishable", keys[0].Type)
	assert.Equal(t, "pk_dev_abc", keys[0].Prefix)
}

func TestClientRevokeKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/environments/env1/keys/revoke", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := api.NewClient("token")
	client.SetBaseURL(server.URL)

	err := client.RevokeKeys("env1")
	require.NoError(t, err)
}

func TestClientNoToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer server.Close()

	client := api.NewClient("")
	client.SetBaseURL(server.URL)

	var result map[string]string
	err := client.Get("/v1/test", &result)
	require.NoError(t, err)
}

func TestClientListSegments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/projects/proj1/segments", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]api.Segment{
			{ID: "seg1", Name: "Pro Users", MatchType: "ALL", Rules: []api.SegmentRule{
				{Attribute: "plan", Operator: "equals", Value: "pro"},
			}},
		})
	}))
	defer server.Close()

	client := api.NewClient("token")
	client.SetBaseURL(server.URL)

	segments, err := client.ListSegments("proj1")
	require.NoError(t, err)
	assert.Len(t, segments, 1)
	assert.Equal(t, "Pro Users", segments[0].Name)
	assert.Equal(t, "ALL", segments[0].MatchType)
	assert.Len(t, segments[0].Rules, 1)
}

func TestClientCreateSegment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/projects/proj1/segments", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(api.Segment{
			ID: "seg2", Name: "Beta Testers", MatchType: "ANY",
		})
	}))
	defer server.Close()

	client := api.NewClient("token")
	client.SetBaseURL(server.URL)

	seg, err := client.CreateSegment("proj1", map[string]any{
		"name": "Beta Testers", "matchType": "ANY",
	})
	require.NoError(t, err)
	assert.Equal(t, "Beta Testers", seg.Name)
}

func TestClientDeleteSegment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/v1/segments/seg1", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := api.NewClient("token")
	client.SetBaseURL(server.URL)

	err := client.DeleteSegment("seg1")
	require.NoError(t, err)
}

func TestClientGetTargetingRules(t *testing.T) {
	segID := "seg1"
	rollout := 50
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/flag-environments/fe1/targeting-rules", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]api.TargetingRule{
			{ID: "tr1", Priority: 0, SegmentID: &segID, ValueOverride: "pro-value", Enabled: true},
			{ID: "tr2", Priority: 1, RolloutPercentage: &rollout, Enabled: true,
				Conditions: []api.TargetingCondition{
					{Attribute: "country", Operator: "equals", Value: "US"},
				}},
		})
	}))
	defer server.Close()

	client := api.NewClient("token")
	client.SetBaseURL(server.URL)

	rules, err := client.GetTargetingRules("fe1")
	require.NoError(t, err)
	assert.Len(t, rules, 2)
	assert.Equal(t, "seg1", *rules[0].SegmentID)
	assert.Equal(t, 50, *rules[1].RolloutPercentage)
	assert.Len(t, rules[1].Conditions, 1)
}

func TestClientSetTargetingRules(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/v1/flag-environments/fe1/targeting-rules", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]api.TargetingRule{
			{ID: "tr1", Priority: 0, Enabled: true, ValueOverride: "catch-all"},
		})
	}))
	defer server.Close()

	client := api.NewClient("token")
	client.SetBaseURL(server.URL)

	result, err := client.SetTargetingRules("fe1", map[string]any{
		"rules": []map[string]any{{"valueOverride": "catch-all", "enabled": true}},
	})
	require.NoError(t, err)
	assert.Len(t, result, 1)
}
