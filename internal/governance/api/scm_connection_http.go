package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/governance/infra"
	"github.com/openware-io/open-green-pass/internal/iam"
	"github.com/openware-io/open-green-pass/internal/platform/githubapp"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type scmConnectionInput struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`
	Token    string `json:"token"`
}
type githubAppConfig struct {
	AppID            string
	ClientID         string
	ClientSecret     string
	InstallURL       string
	PrivateKeyRef    string
	OAuthCallbackURL string
	FrontendURL      string
}

func GitHubAppConfigFromEnv() githubAppConfig {
	return githubAppConfig{
		AppID:            strings.TrimSpace(os.Getenv("GP_GITHUB_APP_ID")),
		ClientID:         strings.TrimSpace(os.Getenv("GP_GITHUB_APP_CLIENT_ID")),
		ClientSecret:     strings.TrimSpace(os.Getenv("GP_GITHUB_APP_CLIENT_SECRET")),
		InstallURL:       strings.TrimSpace(os.Getenv("GP_GITHUB_APP_INSTALL_URL")),
		PrivateKeyRef:    strings.TrimSpace(os.Getenv("GP_GITHUB_APP_PRIVATE_KEY_REF")),
		OAuthCallbackURL: strings.TrimSpace(os.Getenv("GP_GITHUB_APP_OAUTH_CALLBACK_URL")),
		FrontendURL:      strings.TrimRight(strings.TrimSpace(os.Getenv("GP_GITHUB_APP_FRONTEND_URL")), "/"),
	}
}

func (c githubAppConfig) configured() bool {
	return c.AppID != "" && c.ClientID != "" && c.ClientSecret != "" && c.InstallURL != "" && c.PrivateKeyRef != "" && c.OAuthCallbackURL != ""
}

func RegisterSCMConnections(mux *http.ServeMux, store *infra.SCMConnectionStore, namespace string, githubApp githubAppConfig) {
	mux.HandleFunc("GET /scm-connections/github-app", func(w http.ResponseWriter, r *http.Request) {
		if !catalogEditor(w, r) {
			return
		}
		if !githubApp.configured() {
			httpx.WriteJSON(w, 409, map[string]string{"error": "github_app_not_configured", "message": "GitHub App 尚未由平台管理员配置"})
			return
		}
		httpx.WriteJSON(w, 200, map[string]any{"configured": true, "app_id": githubApp.AppID})
	})
	mux.HandleFunc("POST /scm-connections/github-app/authorize", func(w http.ResponseWriter, r *http.Request) {
		if !catalogEditor(w, r) {
			return
		}
		if !githubApp.configured() {
			httpx.WriteJSON(w, http.StatusConflict, map[string]string{"error": "github_app_not_configured"})
			return
		}
		authorizationURL, err := validatedHTTPSURL(githubApp.InstallURL)
		if err != nil {
			httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "github_app_install_url_invalid"})
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"authorization_url": authorizationURL})
	})
	mux.HandleFunc("GET /scm-connections", func(w http.ResponseWriter, r *http.Request) {
		if !catalogEditor(w, r) {
			return
		}
		items, err := store.List(r.Context(), tenantID(r))
		if err != nil {
			httpx.WriteJSON(w, 500, map[string]string{"error": "scm_connections_unavailable"})
			return
		}
		httpx.WriteJSON(w, 200, map[string]any{"connections": items})
	})
	mux.HandleFunc("POST /scm-connections", func(w http.ResponseWriter, r *http.Request) {
		if !catalogEditor(w, r) {
			return
		}
		var in scmConnectionInput
		if httpx.DecodeJSON(r, &in) != nil || strings.TrimSpace(in.Token) == "" || strings.TrimSpace(in.Name) == "" || in.Provider != "github" {
			httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_scm_connection"})
			return
		}
		if err := probeGitHubReadToken(r.Context(), in.Token); err != nil {
			httpx.WriteJSON(w, 502, map[string]string{"error": "github_connection_failed"})
			return
		}
		team := tenantID(r)
		connectionID := store.NextID()
		secretName := fmt.Sprintf("gp-scm-%d-%d", team, connectionID)
		if err := writeSCMSecret(r.Context(), namespace, secretName, in.Token); err != nil {
			httpx.WriteJSON(w, 500, map[string]string{"error": "secret_store_failed"})
			return
		}
		principal, _ := iam.PrincipalFromContext(r.Context())
		now := time.Now()
		item := infra.SCMConnection{ID: connectionID, Provider: "github", Name: strings.TrimSpace(in.Name), CredentialStatus: "verified", VerifiedAt: &now, TokenConfigured: true}
		if err := store.Create(r.Context(), item, "k8s://"+namespace+"/"+secretName+"#token", parseSubject(principal.Subject)); err != nil {
			httpx.WriteJSON(w, 409, map[string]string{"error": "scm_connection_name_conflict"})
			return
		}
		httpx.WriteJSON(w, 201, item)
	})
	mux.HandleFunc("POST /scm-connections/{id}/verify", func(w http.ResponseWriter, r *http.Request) {
		if !catalogEditor(w, r) {
			return
		}
		var connectionID int64
		if _, err := fmt.Sscan(r.PathValue("id"), &connectionID); err != nil || connectionID <= 0 {
			httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_scm_connection"})
			return
		}
		provider, authType, ref, appID, installationID, err := store.CredentialRef(r.Context(), tenantID(r), connectionID)
		if err != nil || provider != "github" {
			httpx.WriteJSON(w, 404, map[string]string{"error": "scm_connection_not_found"})
			return
		}
		var token string
		if authType == "github_app" {
			if appID == "" || installationID == nil {
				err = fmt.Errorf("github app installation missing")
			} else {
				var privateKey string
				privateKey, err = readSCMSecret(r.Context(), ref)
				if err == nil {
					token, err = githubapp.InstallationToken(r.Context(), appID, privateKey, *installationID)
				}
			}
		} else {
			token, err = readSCMSecret(r.Context(), ref)
		}
		if err == nil {
			err = probeGitHubReadToken(r.Context(), token)
		}
		if err != nil {
			_ = store.MarkVerification(r.Context(), tenantID(r), connectionID, "failed", "无法验证 GitHub 只读凭据")
			httpx.WriteJSON(w, 502, map[string]string{"error": "github_connection_failed"})
			return
		}
		if err = store.MarkVerification(r.Context(), tenantID(r), connectionID, "verified", ""); err != nil {
			httpx.WriteJSON(w, 500, map[string]string{"error": "scm_connection_update_failed"})
			return
		}
		httpx.WriteJSON(w, 200, map[string]string{"status": "verified"})
	})
	mux.HandleFunc("GET /scm-connections/github-app/callback", func(w http.ResponseWriter, r *http.Request) {
		if !catalogEditor(w, r) {
			return
		}
		installationID, err := parsePositiveID(r.URL.Query().Get("installation_id"))
		principal, ok := iam.PrincipalFromContext(r.Context())
		actorID, actorErr := parsePositiveID(principal.Subject)
		if err != nil || actorErr != nil || !ok || !githubApp.configured() {
			httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_github_installation"})
			return
		}
		state, err := newGitHubAppAuthorizationState()
		if err != nil {
			httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "authorization_state_create_failed"})
			return
		}
		if err = store.CreateAuthorizationState(r.Context(), tenantID(r), actorID, installationID, state); err != nil {
			httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "authorization_state_store_failed"})
			return
		}
		authorizationURL, err := gitHubUserAuthorizationURL(githubApp.ClientID, githubApp.OAuthCallbackURL, state)
		if err != nil {
			httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "github_app_oauth_url_invalid"})
			return
		}
		http.Redirect(w, r, authorizationURL, http.StatusSeeOther)
	})
	mux.HandleFunc("GET /scm-connections/github-app/oauth-callback", func(w http.ResponseWriter, r *http.Request) {
		if !catalogEditor(w, r) {
			return
		}
		principal, ok := iam.PrincipalFromContext(r.Context())
		actorID, actorErr := parsePositiveID(principal.Subject)
		state := strings.TrimSpace(r.URL.Query().Get("state"))
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if !ok || actorErr != nil || state == "" || code == "" || !githubApp.configured() {
			httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_github_authorization_callback"})
			return
		}
		installationID, err := store.ConsumeAuthorizationState(r.Context(), tenantID(r), actorID, state)
		if err != nil {
			httpx.WriteJSON(w, http.StatusForbidden, map[string]string{"error": "invalid_github_authorization_state"})
			return
		}
		userToken, err := githubapp.ExchangeUserAuthorizationCode(r.Context(), githubApp.ClientID, githubApp.ClientSecret, code)
		if err != nil {
			httpx.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": "github_user_authorization_failed"})
			return
		}
		ownsInstallation, err := githubapp.UserOwnsInstallation(r.Context(), userToken, githubApp.AppID, installationID)
		if err != nil || !ownsInstallation {
			httpx.WriteJSON(w, http.StatusForbidden, map[string]string{"error": "github_installation_not_authorized_for_user"})
			return
		}
		privateKey, err := readSCMSecret(r.Context(), githubApp.PrivateKeyRef)
		if err == nil {
			_, err = githubapp.InstallationToken(r.Context(), githubApp.AppID, privateKey, installationID)
		}
		if err != nil {
			httpx.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": "github_installation_verification_failed"})
			return
		}
		now := time.Now()
		item := infra.SCMConnection{ID: store.NextID(), Provider: "github", Name: fmt.Sprintf("GitHub App 安装 #%d", installationID), CredentialStatus: "verified", VerifiedAt: &now, TokenConfigured: true, AuthType: "github_app", GitHubAppID: githubApp.AppID, InstallationID: &installationID}
		if err := store.Create(r.Context(), item, githubApp.PrivateKeyRef, actorID); err != nil {
			httpx.WriteJSON(w, 409, map[string]string{"error": "github_installation_already_connected"})
			return
		}
		if githubApp.FrontendURL != "" {
			http.Redirect(w, r, githubApp.FrontendURL+"/#/target?github_app=connected", http.StatusSeeOther)
			return
		}
		httpx.WriteJSON(w, 201, item)
	})
}

func newGitHubAppAuthorizationState() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func validatedHTTPSURL(rawURL string) (string, error) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", fmt.Errorf("invalid HTTPS URL")
	}
	return parsed.String(), nil
}

func gitHubUserAuthorizationURL(clientID, callbackURL, state string) (string, error) {
	validatedCallback, err := validatedHTTPSURL(callbackURL)
	if err != nil || strings.TrimSpace(clientID) == "" || strings.TrimSpace(state) == "" {
		return "", fmt.Errorf("invalid GitHub App user authorization configuration")
	}
	query := url.Values{"client_id": {clientID}, "redirect_uri": {validatedCallback}, "state": {state}}
	return "https://github.com/login/oauth/authorize?" + query.Encode(), nil
}

func parsePositiveID(raw string) (int64, error) {
	var v int64
	_, err := fmt.Sscan(raw, &v)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("invalid id")
	}
	return v, nil
}

func probeGitHubReadToken(ctx context.Context, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/rate_limit", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	req.Header.Set("User-Agent", "greenpass-scm-connection")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}
	return nil
}
func writeSCMSecret(ctx context.Context, ns, name, token string) error {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}
	_, err = client.CoreV1().Secrets(ns).Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name}, Type: corev1.SecretTypeOpaque, StringData: map[string]string{"token": token}}, metav1.CreateOptions{})
	return err
}
func readSCMSecret(ctx context.Context, ref string) (string, error) {
	if !strings.HasPrefix(ref, "k8s://") {
		return "", fmt.Errorf("invalid secret ref")
	}
	parts := strings.Split(strings.TrimPrefix(ref, "k8s://"), "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid secret ref")
	}
	nameKey := strings.SplitN(parts[1], "#", 2)
	if len(nameKey) != 2 {
		return "", fmt.Errorf("invalid secret ref")
	}
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return "", err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return "", err
	}
	secret, err := client.CoreV1().Secrets(parts[0]).Get(ctx, nameKey[0], metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	value := secret.Data[nameKey[1]]
	if len(value) == 0 {
		return "", fmt.Errorf("secret key missing")
	}
	return string(value), nil
}
