package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// RFC 8628 §3.5 polling errors returned by PollDeviceToken.
var (
	ErrAuthorizationPending = errors.New("authorization_pending")
	ErrSlowDown             = errors.New("slow_down")
	ErrExpiredToken         = errors.New("expired_token")
	ErrAccessDenied         = errors.New("access_denied")
	ErrInvalidGrant         = errors.New("invalid_grant")
)

var deviceTokenErrors = map[string]error{
	ErrAuthorizationPending.Error(): ErrAuthorizationPending,
	ErrSlowDown.Error():             ErrSlowDown,
	ErrExpiredToken.Error():         ErrExpiredToken,
	ErrAccessDenied.Error():         ErrAccessDenied,
	ErrInvalidGrant.Error():         ErrInvalidGrant,
}

type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type DeviceTokenResult struct {
	User   map[string]any
	Tokens *TokenPair
}

// RequestDeviceCode starts an RFC 8628 device authorization. This endpoint
// uses the standard {code, message} error envelope.
func (c *Client) RequestDeviceCode(deviceID, hostname string) (*DeviceCodeResponse, error) {
	resp, err := c.postDevice("/v1/auth/device/code", map[string]string{
		"deviceId": deviceID,
		"hostname": hostname,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var apiErr struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
			return nil, fmt.Errorf("API error %d", resp.StatusCode)
		}
		return nil, &APIError{StatusCode: resp.StatusCode, Code: apiErr.Code, Message: apiErr.Message}
	}

	var result DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode device code response: %w", err)
	}
	return &result, nil
}

// PollDeviceToken polls once for the device's tokens. A 400 is decoded as the
// RFC 8628 envelope {"error": "..."} and mapped to one of the Err* sentinels;
// an unrecognized value is returned as a generic error so callers never loop
// on an unexpected response. It does not go through do/doOnce, which assume
// the {code, message} envelope, and it never retries — the caller owns the
// polling loop.
func (c *Client) PollDeviceToken(deviceCode string) (*DeviceTokenResult, error) {
	resp, err := c.postDevice("/v1/auth/device/token", map[string]string{
		"device_code": deviceCode,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		var result AuthResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode device token response: %w", err)
		}
		return &DeviceTokenResult{User: result.User, Tokens: &result.Tokens}, nil
	case resp.StatusCode == http.StatusBadRequest:
		var body struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return nil, fmt.Errorf("failed to decode device token error: %w", err)
		}
		if sentinel, ok := deviceTokenErrors[body.Error]; ok {
			return nil, sentinel
		}
		return nil, fmt.Errorf("unexpected device token error %q", body.Error)
	default:
		return nil, fmt.Errorf("device token request failed: API error %d", resp.StatusCode)
	}
}

func (c *Client) postDevice(path string, body any) (*http.Response, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Flagify-Source", "cli")

	return c.httpClient.Do(req)
}
