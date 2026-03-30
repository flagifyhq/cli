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
