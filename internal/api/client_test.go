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
