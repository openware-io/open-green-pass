// Package githubapp implements the minimal GitHub App server-to-server flow.
// It intentionally uses the standard library so tokens and private keys do not
// pass through an external SDK or the browser.
package githubapp

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const apiRoot = "https://api.github.com"

func InstallationToken(ctx context.Context, appID, privateKey string, installationID int64) (string, error) {
	jwt, err := appJWT(appID, privateKey, time.Now())
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/app/installations/%d/access_tokens", apiRoot, installationID), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("User-Agent", "greenpass-github-app")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("GitHub App installation token request returned HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 128*1024)).Decode(&payload); err != nil || strings.TrimSpace(payload.Token) == "" {
		return "", fmt.Errorf("GitHub App did not return an installation token")
	}
	return payload.Token, nil
}

// InstallationAccountLogin resolves the account/org login for an installation
// so operators can identify a connection without memorizing installation IDs.
func InstallationAccountLogin(ctx context.Context, appID, privateKey string, installationID int64) (string, error) {
	jwt, err := appJWT(appID, privateKey, time.Now())
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/app/installations/%d", apiRoot, installationID), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("User-Agent", "greenpass-github-app")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("GitHub App installation details request returned HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Account struct {
			Login string `json:"login"`
		} `json:"account"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 128*1024)).Decode(&payload); err != nil {
		return "", fmt.Errorf("GitHub App installation details response is invalid")
	}
	return strings.TrimSpace(payload.Account.Login), nil
}

// ExchangeUserAuthorizationCode trades a GitHub App user-authorization code
// for a short-lived user token. The token is used only for installation
// ownership verification and is never persisted by GreenPass.
func ExchangeUserAuthorizationCode(ctx context.Context, clientID, clientSecret, code string) (string, error) {
	form := url.Values{"client_id": {strings.TrimSpace(clientID)}, "client_secret": {strings.TrimSpace(clientSecret)}, "code": {strings.TrimSpace(code)}}
	if form.Get("client_id") == "" || form.Get("client_secret") == "" || form.Get("code") == "" {
		return "", fmt.Errorf("GitHub App user authorization response is incomplete")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "greenpass-github-app")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("GitHub App user authorization token request returned HTTP %d", resp.StatusCode)
	}
	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 128*1024)).Decode(&payload); err != nil || strings.TrimSpace(payload.AccessToken) == "" {
		return "", fmt.Errorf("GitHub App user authorization did not return an access token")
	}
	return payload.AccessToken, nil
}

// UserOwnsInstallation verifies the installation belongs to the GitHub user
// who just completed the App authorization. GitHub's setup URL installation_id
// is otherwise spoofable by an arbitrary request.
func UserOwnsInstallation(ctx context.Context, userToken, appID string, installationID int64) (bool, error) {
	if strings.TrimSpace(userToken) == "" || installationID <= 0 {
		return false, fmt.Errorf("GitHub App installation verification input is invalid")
	}
	expectedAppID, err := strconv.ParseInt(strings.TrimSpace(appID), 10, 64)
	if err != nil {
		return false, fmt.Errorf("GitHub App ID is invalid")
	}
	for page := 1; page <= 10; page++ {
		endpoint := fmt.Sprintf("%s/user/installations?per_page=100&page=%d", apiRoot, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return false, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(userToken))
		req.Header.Set("User-Agent", "greenpass-github-app")
		resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
		if err != nil {
			return false, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return false, fmt.Errorf("GitHub user installations request returned HTTP %d", resp.StatusCode)
		}
		var payload struct {
			Installations []struct {
				ID    int64 `json:"id"`
				AppID int64 `json:"app_id"`
			} `json:"installations"`
		}
		decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 512*1024)).Decode(&payload)
		resp.Body.Close()
		if decodeErr != nil {
			return false, fmt.Errorf("GitHub user installations response is invalid")
		}
		for _, installation := range payload.Installations {
			if installation.ID == installationID && installation.AppID == expectedAppID {
				return true, nil
			}
		}
		if len(payload.Installations) < 100 {
			return false, nil
		}
	}
	return false, fmt.Errorf("GitHub user installation list exceeded supported page limit")
}

func appJWT(appID, privateKey string, now time.Time) (string, error) {
	if _, err := strconv.ParseInt(strings.TrimSpace(appID), 10, 64); err != nil {
		return "", fmt.Errorf("GitHub App ID is invalid")
	}
	block, _ := pem.Decode([]byte(privateKey))
	if block == nil {
		return "", fmt.Errorf("GitHub App private key is not PEM")
	}
	keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		if key, legacyErr := x509.ParsePKCS1PrivateKey(block.Bytes); legacyErr == nil {
			keyAny = key
		} else {
			return "", fmt.Errorf("GitHub App private key is invalid")
		}
	}
	key, ok := keyAny.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("GitHub App private key must be RSA")
	}
	encode := base64.RawURLEncoding.EncodeToString
	header := encode([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload, _ := json.Marshal(map[string]any{"iat": now.Add(-30 * time.Second).Unix(), "exp": now.Add(9 * time.Minute).Unix(), "iss": strings.TrimSpace(appID)})
	signingInput := header + "." + encode(payload)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + encode(signature), nil
}
