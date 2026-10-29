package config

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

type AuthStatus string

const (
	AuthStatusActive      AuthStatus = "active"
	AuthStatusRefreshable AuthStatus = "refreshable"
	AuthStatusExpired     AuthStatus = "expired"
	AuthStatusInvalid     AuthStatus = "invalid"
	AuthStatusLoggedOut   AuthStatus = "logged-out"
)

type localTokenState int

const (
	localTokenMissing localTokenState = iota
	localTokenActive
	localTokenExpired
	localTokenInvalid
)

type localTokenClaims struct {
	Type     string `json:"type"`
	Exp      int64  `json:"exp"`
	ID       string `json:"jti"`
	UserID   string `json:"uid"`
	DeviceID string `json:"did"`
}

// AccountAuthStatus classifies only locally inspectable token metadata. It does
// not verify signatures or claim that a server-side session has not been revoked.
func AccountAuthStatus(account *Account, now time.Time) AuthStatus {
	if account == nil || (account.AccessToken == "" && account.RefreshToken == "") {
		return AuthStatusLoggedOut
	}

	accessState := inspectLocalToken(account.AccessToken, "access", now)
	refreshState := inspectLocalToken(account.RefreshToken, "refresh", now)
	if accessState == localTokenActive {
		return AuthStatusActive
	}
	if refreshState == localTokenActive {
		return AuthStatusRefreshable
	}
	if accessState == localTokenInvalid || refreshState == localTokenInvalid {
		return AuthStatusInvalid
	}
	return AuthStatusExpired
}

func inspectLocalToken(token, expectedType string, now time.Time) localTokenState {
	if token == "" {
		return localTokenMissing
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return localTokenInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return localTokenInvalid
	}
	var claims localTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return localTokenInvalid
	}
	if expectedType == "refresh" && claims.Type == "cli_refresh" && claims.Exp == 0 && claims.ID != "" && claims.UserID != "" && claims.DeviceID != "" {
		return localTokenActive
	}
	if claims.Type != expectedType || claims.Exp <= 0 {
		return localTokenInvalid
	}
	if claims.Exp <= now.Unix() {
		return localTokenExpired
	}
	return localTokenActive
}

// IsPersistentCredential inspects metadata only, never server validity.
func IsPersistentCredential(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	var claims localTokenClaims
	return json.Unmarshal(payload, &claims) == nil && claims.Type == "cli_refresh"
}
