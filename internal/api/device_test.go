package api_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flagifyhq/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDeviceTestClient(t *testing.T, handler http.HandlerFunc) *api.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := api.NewClient("")
	client.SetBaseURL(server.URL)
	return client
}

func TestRequestDeviceCode(t *testing.T) {
	client := newDeviceTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/auth/device/code", r.URL.Path)
		assert.Equal(t, "cli", r.Header.Get("X-Flagify-Source"))
		assert.Empty(t, r.Header.Get("Authorization"))

		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, map[string]string{"deviceId": "cli-abc", "hostname": "build-box"}, body)

		json.NewEncoder(w).Encode(map[string]any{
			"device_code":               "secret",
			"user_code":                 "BCDF-GHJK",
			"verification_uri":          "https://console.test/device",
			"verification_uri_complete": "https://console.test/device?code=BCDF-GHJK",
			"expires_in":                600,
			"interval":                  5,
		})
	})

	resp, err := client.RequestDeviceCode("cli-abc", "build-box")
	require.NoError(t, err)
	assert.Equal(t, &api.DeviceCodeResponse{
		DeviceCode:              "secret",
		UserCode:                "BCDF-GHJK",
		VerificationURI:         "https://console.test/device",
		VerificationURIComplete: "https://console.test/device?code=BCDF-GHJK",
		ExpiresIn:               600,
		Interval:                5,
	}, resp)
}

func TestRequestDeviceCodeStandardError(t *testing.T) {
	client := newDeviceTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{"code": "too_many_requests", "message": "rate limit exceeded"})
	})

	_, err := client.RequestDeviceCode("cli-abc", "build-box")
	var apiErr *api.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusTooManyRequests, apiErr.StatusCode)
	assert.Equal(t, "too_many_requests", apiErr.Code)
}

func TestRequestDeviceCodeNetworkError(t *testing.T) {
	client := api.NewClient("")
	client.SetBaseURL("http://127.0.0.1:1")

	_, err := client.RequestDeviceCode("cli-abc", "build-box")
	require.Error(t, err)
}

func TestPollDeviceTokenSuccess(t *testing.T) {
	client := newDeviceTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/auth/device/token", r.URL.Path)
		assert.Equal(t, "cli", r.Header.Get("X-Flagify-Source"))
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "secret", body["device_code"])

		json.NewEncoder(w).Encode(map[string]any{
			"user":   map[string]any{"email": "jane@acme.com"},
			"tokens": map[string]string{"accessToken": "at", "refreshToken": "rt"},
		})
	})

	result, err := client.PollDeviceToken("secret")
	require.NoError(t, err)
	assert.Equal(t, "jane@acme.com", result.User["email"])
	assert.Equal(t, &api.TokenPair{AccessToken: "at", RefreshToken: "rt"}, result.Tokens)
}

func TestPollDeviceTokenRFCErrors(t *testing.T) {
	cases := map[string]error{
		"authorization_pending": api.ErrAuthorizationPending,
		"slow_down":             api.ErrSlowDown,
		"expired_token":         api.ErrExpiredToken,
		"access_denied":         api.ErrAccessDenied,
		"invalid_grant":         api.ErrInvalidGrant,
	}
	for code, want := range cases {
		t.Run(code, func(t *testing.T) {
			client := newDeviceTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": code})
			})

			_, err := client.PollDeviceToken("secret")
			assert.ErrorIs(t, err, want)
		})
	}
}

func TestPollDeviceTokenUnknownErrorIsNotASentinel(t *testing.T) {
	client := newDeviceTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "unsupported_grant_type"})
	})

	_, err := client.PollDeviceToken("secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported_grant_type")
	for _, sentinel := range []error{api.ErrAuthorizationPending, api.ErrSlowDown, api.ErrExpiredToken, api.ErrAccessDenied, api.ErrInvalidGrant} {
		assert.False(t, errors.Is(err, sentinel))
	}
}

func TestPollDeviceTokenServerError(t *testing.T) {
	client := newDeviceTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"code": "internal_error", "message": "something went wrong"})
	})

	_, err := client.PollDeviceToken("secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestPollDeviceTokenMalformedJSON(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusBadRequest} {
		client := newDeviceTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			w.Write([]byte("{not json"))
		})

		_, err := client.PollDeviceToken("secret")
		assert.Error(t, err, "status %d", status)
	}
}

func TestPollDeviceTokenNetworkError(t *testing.T) {
	client := api.NewClient("")
	client.SetBaseURL("http://127.0.0.1:1")

	_, err := client.PollDeviceToken("secret")
	require.Error(t, err)
}
