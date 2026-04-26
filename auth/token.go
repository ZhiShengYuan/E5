package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"e5autocaller/notifier"
)

const tokenEndpoint = "https://login.microsoftonline.com/common/oauth2/v2.0/token"

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// OnTokenUpdated is called when a new refresh_token is obtained.
type OnTokenUpdated func(newRefreshToken string)

type TokenManager struct {
	clientID     string
	clientSecret string
	refreshToken string
	redirectURI  string
	appNum       int

	accessToken string
	expiresAt   time.Time
	notifier    notifier.Notifier
	onUpdate    OnTokenUpdated
}

func NewTokenManager(clientID, clientSecret, refreshToken, redirectURI string, appNum int, n notifier.Notifier, onUpdate OnTokenUpdated) *TokenManager {
	return &TokenManager{
		clientID:     clientID,
		clientSecret: clientSecret,
		refreshToken: refreshToken,
		redirectURI:  redirectURI,
		appNum:       appNum,
		notifier:     n,
		onUpdate:     onUpdate,
	}
}

func (tm *TokenManager) Refresh() (string, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", tm.refreshToken)
	data.Set("client_id", tm.clientID)
	data.Set("client_secret", tm.clientSecret)
	data.Set("redirect_uri", tm.redirectURI)

	resp, err := http.Post(tokenEndpoint, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		msg := fmt.Sprintf("[E5Autocaller] Token request failed for account %d: %v", tm.appNum, err)
		fmt.Println(msg)
		tm.notifier.Notify(msg)
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		msg := fmt.Sprintf("[E5Autocaller] Decode token response failed for account %d: %v", tm.appNum, err)
		fmt.Println(msg)
		tm.notifier.Notify(msg)
		return "", fmt.Errorf("decode token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		msg := fmt.Sprintf("[E5Autocaller] Access token not received for account %d, check credentials", tm.appNum)
		fmt.Println(msg)
		tm.notifier.Notify(msg)
		return "", fmt.Errorf("access token not received, check credentials")
	}

	tm.accessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		tm.refreshToken = tokenResp.RefreshToken
		if tm.onUpdate != nil {
			tm.onUpdate(tm.refreshToken)
		}
	}
	tm.expiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return tm.accessToken, nil
}

func (tm *TokenManager) GetAccessToken() (string, error) {
	if tm.accessToken != "" && time.Now().Before(tm.expiresAt.Add(-5*time.Minute)) {
		return tm.accessToken, nil
	}
	return tm.Refresh()
}

func (tm *TokenManager) GetRefreshToken() string {
	return tm.refreshToken
}

func (tm *TokenManager) AppNum() int {
	return tm.appNum
}
