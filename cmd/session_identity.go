package cmd

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/flagifyhq/cli/internal/api"
	"github.com/flagifyhq/cli/internal/config"
)

// sameProfileSession is a local consistency guard, not JWT authentication.
// The server still verifies signatures. A command must never substitute a
// different principal or send replacement credentials to its pinned API URL.
func sameProfileSession(original, current *config.Account) bool {
	if sessionAPIEndpoint(original.APIUrl) != sessionAPIEndpoint(current.APIUrl) || original.DeviceID != current.DeviceID {
		return false
	}
	if original.RefreshToken == current.RefreshToken {
		return true
	}
	originalUser, originalDevice, originalOK := refreshIdentity(original.RefreshToken)
	currentUser, currentDevice, currentOK := refreshIdentity(current.RefreshToken)
	return originalOK && currentOK && originalUser == currentUser && originalDevice == currentDevice
}

func sessionAPIEndpoint(endpoint string) string {
	if endpoint == "" {
		endpoint = api.DefaultBaseURL
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = strings.TrimRight(parsed.RawPath, "/")
	return parsed.String()
}

func refreshIdentity(token string) (userID, deviceID string, ok bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", "", false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", false
	}
	var claims struct {
		Type     string `json:"type"`
		UserID   string `json:"uid"`
		DeviceID string `json:"did"`
	}
	if json.Unmarshal(payload, &claims) != nil || (claims.Type != "refresh" && claims.Type != "cli_refresh") || claims.UserID == "" || claims.DeviceID == "" {
		return "", "", false
	}
	return claims.UserID, claims.DeviceID, true
}
