package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DeviceFlowConfig holds the OAuth app configuration for device flow auth.
type DeviceFlowConfig struct {
	// ClientID is the GitHub OAuth App client ID.
	// Users can register their own app at https://github.com/settings/applications/new
	// with "Device flow" enabled. No client secret needed for public clients.
	ClientID string
	Scopes   []string
}

// DeviceCodeResponse is the initial response from GitHub's device flow endpoint.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// AccessTokenResponse is the final response with the token.
type AccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

var authHTTPClient = &http.Client{Timeout: 15 * time.Second}

// RequestDeviceCode initiates the device flow by requesting a device code from GitHub.
func RequestDeviceCode(ctx context.Context, cfg DeviceFlowConfig) (*DeviceCodeResponse, error) {
	data := url.Values{
		"client_id": {cfg.ClientID},
		"scope":     {strings.Join(cfg.Scopes, " ")},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://github.com/login/device/code", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := authHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting device code: %w", err)
	}
	defer closeResponseBody(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code request failed (%d): %s", resp.StatusCode, string(body))
	}

	var result DeviceCodeResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing device code response: %w", err)
	}

	return &result, nil
}

// PollForToken polls GitHub until the user authorizes the device, the code expires, or an error occurs.
// The onPoll callback is called before each poll attempt (for displaying progress).
func PollForToken(ctx context.Context, cfg DeviceFlowConfig, deviceCode *DeviceCodeResponse, onPoll func(attempt int)) (*AccessTokenResponse, error) {
	interval := time.Duration(deviceCode.Interval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}

	deadline := time.Now().Add(time.Duration(deviceCode.ExpiresIn) * time.Second)
	attempt := 0

	for time.Now().Before(deadline) {
		attempt++
		if onPoll != nil {
			onPoll(attempt)
		}

		select {
		case <-time.After(interval):
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		data := url.Values{
			"client_id":   {cfg.ClientID},
			"device_code": {deviceCode.DeviceCode},
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		}

		req, err := http.NewRequestWithContext(ctx, "POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
		if err != nil {
			return nil, fmt.Errorf("creating poll request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		resp, err := authHTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("polling for token: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		closeResponseBody(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("reading poll response: %w", err)
		}

		var result AccessTokenResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("parsing poll response: %w", err)
		}

		switch result.Error {
		case "":
			// Success
			return &result, nil
		case "authorization_pending":
			// User hasn't authorized yet, keep polling
			continue
		case "slow_down":
			// Must add 5 seconds to the interval per GitHub docs
			interval += 5 * time.Second
			continue
		case "expired_token":
			return nil, fmt.Errorf("device code expired — please try again")
		case "access_denied":
			return nil, fmt.Errorf("authorization denied by user")
		case "unsupported_grant_type":
			return nil, fmt.Errorf("OAuth error: unsupported grant type (this is a bug in pr-wrangler)")
		case "incorrect_client_credentials":
			return nil, fmt.Errorf("OAuth error: invalid client ID — check PR_WRANGLER_CLIENT_ID env var or oauth_client_id in config file")
		case "incorrect_device_code":
			return nil, fmt.Errorf("OAuth error: invalid device code (this is a bug in pr-wrangler)")
		case "device_flow_disabled":
			return nil, fmt.Errorf("device flow is not enabled for this OAuth App — enable it in the app settings on GitHub")
		default:
			return nil, fmt.Errorf("OAuth error: %s — %s", result.Error, result.ErrorDesc)
		}
	}

	return nil, fmt.Errorf("device code expired — please try again")
}

// FetchAuthenticatedUser fetches the username for the given token.
func FetchAuthenticatedUser(ctx context.Context, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "pr-wrangler")

	resp, err := authHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching user: %w", err)
	}
	defer closeResponseBody(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch user (%d)", resp.StatusCode)
	}

	var user struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", fmt.Errorf("parsing user response: %w", err)
	}

	return user.Login, nil
}

func closeResponseBody(body io.Closer) {
	if err := body.Close(); err != nil {
		_ = err
	}
}
