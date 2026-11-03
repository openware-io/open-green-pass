package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	aiinfra "github.com/openware-io/open-green-pass/internal/ai/infra"
	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/governance/infra"
	"github.com/openware-io/open-green-pass/internal/iam"
	"github.com/openware-io/open-green-pass/internal/platform/githubapp"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
)

// SCMAuditFunc lets composition inject the trusted audit service without
// making the governance API package depend on the trusted domain.
type SCMAuditFunc func(context.Context, int64, string, string, int64, map[string]any) error

type scmConnectionInput struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`
	Token    string `json:"token"`
}

type scmConnectionRotateInput struct {
	Token string `json:"token"`
}

type GitHubAppConfig struct {
	AppID            string
	ClientID         string
	ClientSecret     string
	InstallURL       string
	PrivateKeyRef    string
	OAuthCallbackURL string
	FrontendURL      string
}

type githubTokenInfo struct {
	Login  string
	Scopes []string
}

// githubAPIBase and githubHTTPClient are variables so focused unit tests can
// use an in-memory transport without opening a local TCP listener.
var (
	githubAPIBase    = "https://api.github.com"
	githubHTTPClient = &http.Client{Timeout: 15 * time.Second}
)

func GitHubAppConfigFromEnv() GitHubAppConfig {
	return GitHubAppConfig{
		AppID:            strings.TrimSpace(os.Getenv("GP_GITHUB_APP_ID")),
		ClientID:         strings.TrimSpace(os.Getenv("GP_GITHUB_APP_CLIENT_ID")),
		ClientSecret:     strings.TrimSpace(os.Getenv("GP_GITHUB_APP_CLIENT_SECRET")),
		InstallURL:       strings.TrimSpace(os.Getenv("GP_GITHUB_APP_INSTALL_URL")),
		PrivateKeyRef:    strings.TrimSpace(os.Getenv("GP_GITHUB_APP_PRIVATE_KEY_REF")),
		OAuthCallbackURL: strings.TrimSpace(os.Getenv("GP_GITHUB_APP_OAUTH_CALLBACK_URL")),
		FrontendURL:      strings.TrimRight(strings.TrimSpace(os.Getenv("GP_GITHUB_APP_FRONTEND_URL")), "/"),
	}
}

func (c GitHubAppConfig) configured() bool {
	return c.AppID != "" && c.ClientID != "" && c.ClientSecret != "" && c.InstallURL != "" &&
		c.PrivateKeyRef != "" && c.OAuthCallbackURL != "" && c.FrontendURL != ""
}

// scmHandlers groups the SCM connection endpoints. Handlers are methods so
// registration stays a flat route table and every endpoint keeps its own
// bounded complexity.
type scmHandlers struct {
	store     *infra.SCMConnectionStore
	namespace string
	githubApp GitHubAppConfig
	audit     SCMAuditFunc
}

func RegisterSCMConnections(mux *http.ServeMux, store *infra.SCMConnectionStore, namespace string, githubApp GitHubAppConfig, audit SCMAuditFunc) {
	h := &scmHandlers{store: store, namespace: namespace, githubApp: githubApp, audit: audit}
	mux.HandleFunc("GET /scm-connections/github-app", h.getGitHubApp)
	mux.HandleFunc("POST /scm-connections/github-app/authorize", h.authorizeGitHubApp)
	mux.HandleFunc("GET /scm-connections", h.listConnections)
	mux.HandleFunc("POST /scm-connections", h.createConnection)
	mux.HandleFunc("POST /scm-connections/{id}/verify", h.verifyConnection)
	mux.HandleFunc("POST /scm-connections/{id}/rotate", h.rotateConnection)
	mux.HandleFunc("DELETE /scm-connections/{id}", h.deleteConnection)
	mux.HandleFunc("GET /scm-connections/github-app/callback", h.githubAppInstallCallback)
	mux.HandleFunc("GET /scm-connections/github-app/oauth-callback", h.githubAppOAuthCallback)
}

func (h *scmHandlers) getGitHubApp(w http.ResponseWriter, r *http.Request) {
	if !catalogEditor(w, r) {
		return
	}
	if !h.githubApp.configured() {
		httpx.WriteJSON(w, 409, map[string]string{"error": "github_app_not_configured", "message": "GitHub App 尚未由平台管理员完成配置"})
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"configured": true, "app_id": h.githubApp.AppID})
}

func (h *scmHandlers) authorizeGitHubApp(w http.ResponseWriter, r *http.Request) {
	if !catalogEditor(w, r) {
		return
	}
	if !h.githubApp.configured() {
		httpx.WriteJSON(w, http.StatusConflict, map[string]string{"error": "github_app_not_configured"})
		return
	}
	authorizationURL, err := validatedHTTPSURL(h.githubApp.InstallURL)
	if err != nil {
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "github_app_install_url_invalid"})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"authorization_url": authorizationURL})
}

func (h *scmHandlers) listConnections(w http.ResponseWriter, r *http.Request) {
	if !catalogEditor(w, r) {
		return
	}
	items, err := h.store.List(r.Context(), tenantID(r))
	if err != nil {
		httpx.WriteJSON(w, 500, map[string]string{"error": "scm_connections_unavailable"})
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"connections": items})
}

func (h *scmHandlers) createConnection(w http.ResponseWriter, r *http.Request) {
	if !catalogEditor(w, r) {
		return
	}
	var in scmConnectionInput
	if httpx.DecodeJSON(r, &in) != nil || strings.TrimSpace(in.Token) == "" || strings.TrimSpace(in.Name) == "" || in.Provider != "github" {
		httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_scm_connection"})
		return
	}
	if _, err := probeGitHubReadToken(r.Context(), strings.TrimSpace(in.Token), "pat"); err != nil {
		httpx.WriteJSON(w, 502, map[string]string{"error": "github_connection_failed", "message": "仅支持可读取仓库的 fine-grained 只读 PAT；请勿使用 classic 或带写权限的 Token"})
		return
	}
	team := tenantID(r)
	connectionID := h.store.NextID()
	secretName := fmt.Sprintf("gp-scm-%d-%d", team, connectionID)
	if err := writeSCMSecret(r.Context(), h.namespace, secretName, in.Token, true); err != nil {
		slog.Error("create scm secret failed", "connection_id", connectionID, "error", err)
		httpx.WriteJSON(w, 500, map[string]string{"error": "secret_store_failed"})
		return
	}
	principal, _ := iam.PrincipalFromContext(r.Context())
	now := time.Now()
	item := infra.SCMConnection{ID: connectionID, Provider: "github", Name: strings.TrimSpace(in.Name), CredentialStatus: "verified", VerifiedAt: &now, TokenConfigured: true, AuthType: "pat"}
	if err := h.store.Create(r.Context(), item, "k8s://"+h.namespace+"/"+secretName+"#token", actorIDFromSubject(principal.Subject)); err != nil {
		// Never leave a credential behind if the metadata write failed.
		_ = deleteSCMSecret(r.Context(), h.namespace, secretName)
		if isUniqueViolation(err) {
			httpx.WriteJSON(w, 409, map[string]string{"error": "scm_connection_name_conflict"})
			return
		}
		slog.Error("create scm connection failed", "connection_id", connectionID, "error", err)
		httpx.WriteJSON(w, 500, map[string]string{"error": "scm_connection_create_failed"})
		return
	}
	emitSCMAudit(r.Context(), h.audit, principal, "scm.connection.created", "scm_connection", item.ID, map[string]any{
		"name": item.Name, "provider": item.Provider, "auth_type": item.AuthType, "credential_status": item.CredentialStatus,
	})
	httpx.WriteJSON(w, 201, item)
}

func (h *scmHandlers) verifyConnection(w http.ResponseWriter, r *http.Request) {
	if !catalogEditor(w, r) {
		return
	}
	connectionID, ok := parseConnectionID(r)
	if !ok {
		httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_scm_connection"})
		return
	}
	teamID := tenantID(r)
	provider, authType, ref, appID, installationID, err := h.store.CredentialRef(r.Context(), teamID, connectionID)
	if err != nil || provider != "github" {
		httpx.WriteJSON(w, 404, map[string]string{"error": "scm_connection_not_found"})
		return
	}
	boundRepos, err := h.verifyConnectionCredential(r.Context(), teamID, connectionID, authType, ref, appID, installationID)
	principal, _ := iam.PrincipalFromContext(r.Context())
	if err != nil {
		_ = h.store.MarkVerification(r.Context(), teamID, connectionID, "failed", "无法验证 GitHub 只读凭据或绑定仓库的 Contents 权限")
		emitSCMAudit(r.Context(), h.audit, principal, "scm.connection.verify_failed", "scm_connection", connectionID, map[string]any{
			"provider": provider, "auth_type": authType, "bound_repository_count": len(boundRepos),
		})
		httpx.WriteJSON(w, 502, map[string]string{"error": "github_connection_failed", "message": "Token 或已绑定仓库的读取权限验证失败"})
		return
	}
	if err = h.store.MarkVerification(r.Context(), teamID, connectionID, "verified", ""); err != nil {
		httpx.WriteJSON(w, 500, map[string]string{"error": "scm_connection_update_failed"})
		return
	}
	emitSCMAudit(r.Context(), h.audit, principal, "scm.connection.verified", "scm_connection", connectionID, map[string]any{
		"provider": provider, "auth_type": authType, "bound_repository_count": len(boundRepos),
	})
	httpx.WriteJSON(w, 200, map[string]string{"status": "verified"})
}

// verifyConnectionCredential resolves the stored credential, probes GitHub with
// it, then proves every bound repository is readable.
func (h *scmHandlers) verifyConnectionCredential(ctx context.Context, teamID, connectionID int64, authType, ref, appID string, installationID *int64) ([]string, error) {
	token, err := resolveSCMConnectionToken(ctx, authType, ref, appID, installationID)
	boundRepos := make([]string, 0)
	if err == nil {
		_, err = probeGitHubReadToken(ctx, token, authType)
	}
	if err == nil {
		boundRepos, err = h.store.BoundRepositoryURLs(ctx, teamID, connectionID)
	}
	if err != nil {
		return boundRepos, err
	}
	for _, repoURL := range boundRepos {
		if validateErr := aiinfra.ValidateGitHubRepository(ctx, token, repoURL); validateErr != nil {
			return boundRepos, fmt.Errorf("repository %s: %w", repoURL, validateErr)
		}
	}
	return boundRepos, nil
}

func (h *scmHandlers) rotateConnection(w http.ResponseWriter, r *http.Request) {
	if !catalogEditor(w, r) {
		return
	}
	connectionID, ok := parseConnectionID(r)
	if !ok {
		httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_scm_connection"})
		return
	}
	var in scmConnectionRotateInput
	if httpx.DecodeJSON(r, &in) != nil || strings.TrimSpace(in.Token) == "" {
		httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_scm_connection"})
		return
	}
	provider, authType, ref, _, _, err := h.store.CredentialRef(r.Context(), tenantID(r), connectionID)
	if err != nil || provider != "github" {
		httpx.WriteJSON(w, 404, map[string]string{"error": "scm_connection_not_found"})
		return
	}
	if authType != "pat" {
		httpx.WriteJSON(w, 409, map[string]string{"error": "github_app_rotation_not_supported", "message": "GitHub App 连接通过平台私钥轮换，不支持在连接上直接替换 Token"})
		return
	}
	if _, err := probeGitHubReadToken(r.Context(), strings.TrimSpace(in.Token), "pat"); err != nil {
		httpx.WriteJSON(w, 502, map[string]string{"error": "github_connection_failed", "message": "新 Token 未通过只读验证"})
		return
	}
	if err := updateSCMSecret(r.Context(), ref, in.Token); err != nil {
		slog.Error("rotate scm secret failed", "connection_id", connectionID, "error", err)
		httpx.WriteJSON(w, 500, map[string]string{"error": "secret_store_failed"})
		return
	}
	if err := h.store.MarkVerification(r.Context(), tenantID(r), connectionID, "verified", ""); err != nil {
		httpx.WriteJSON(w, 500, map[string]string{"error": "scm_connection_update_failed"})
		return
	}
	principal, _ := iam.PrincipalFromContext(r.Context())
	emitSCMAudit(r.Context(), h.audit, principal, "scm.connection.rotated", "scm_connection", connectionID, map[string]any{
		"provider": provider, "auth_type": authType,
	})
	httpx.WriteJSON(w, 200, map[string]string{"status": "rotated"})
}

func (h *scmHandlers) deleteConnection(w http.ResponseWriter, r *http.Request) {
	if !catalogEditor(w, r) {
		return
	}
	connectionID, ok := parseConnectionID(r)
	if !ok {
		httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_scm_connection"})
		return
	}
	item, secretRef, err := h.store.Delete(r.Context(), tenantID(r), connectionID)
	if errors.Is(err, infra.ErrSCMConnectionNotFound) {
		httpx.WriteJSON(w, 404, map[string]string{"error": "scm_connection_not_found"})
		return
	}
	if err != nil {
		slog.Error("delete scm connection failed", "connection_id", connectionID, "error", err)
		httpx.WriteJSON(w, 500, map[string]string{"error": "scm_connection_delete_failed"})
		return
	}
	// PAT secrets are owned by exactly one connection. GitHub App
	// connections share the platform private-key Secret, so never delete it.
	if item.AuthType == "pat" && secretRef != "" {
		h.deleteCredentialSecret(r.Context(), connectionID, secretRef)
	}
	principal, _ := iam.PrincipalFromContext(r.Context())
	emitSCMAudit(r.Context(), h.audit, principal, "scm.connection.deleted", "scm_connection", connectionID, map[string]any{
		"name": item.Name, "provider": item.Provider, "auth_type": item.AuthType,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *scmHandlers) deleteCredentialSecret(ctx context.Context, connectionID int64, secretRef string) {
	namespace, name, _, ok := parseKubernetesSecretRef(secretRef)
	if !ok {
		return
	}
	if err := deleteSCMSecret(ctx, namespace, name); err != nil {
		slog.Error("delete scm secret failed", "connection_id", connectionID, "error", err)
	}
}

func (h *scmHandlers) githubAppInstallCallback(w http.ResponseWriter, r *http.Request) {
	if !catalogEditor(w, r) {
		return
	}
	installationID, err := parsePositiveID(r.URL.Query().Get("installation_id"))
	principal, ok := iam.PrincipalFromContext(r.Context())
	actorID := actorIDFromSubject(principal.Subject)
	if err != nil || actorID <= 0 || !ok || !h.githubApp.configured() {
		httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_github_installation"})
		return
	}
	state, err := newGitHubAppAuthorizationState()
	if err != nil {
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "authorization_state_create_failed"})
		return
	}
	if err = h.store.CreateAuthorizationState(r.Context(), tenantID(r), actorID, installationID, state); err != nil {
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "authorization_state_store_failed"})
		return
	}
	authorizationURL, err := gitHubUserAuthorizationURL(h.githubApp.ClientID, h.githubApp.OAuthCallbackURL, state)
	if err != nil {
		httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "github_app_oauth_url_invalid"})
		return
	}
	http.Redirect(w, r, authorizationURL, http.StatusSeeOther)
}

func (h *scmHandlers) githubAppOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if !catalogEditor(w, r) {
		return
	}
	principal, actorID, state, code, ok := h.oauthCallbackParams(r)
	if !ok {
		httpx.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_github_authorization_callback"})
		return
	}
	installationID, err := h.store.ConsumeAuthorizationState(r.Context(), tenantID(r), actorID, state)
	if err != nil {
		httpx.WriteJSON(w, http.StatusForbidden, map[string]string{"error": "invalid_github_authorization_state"})
		return
	}
	userToken, err := githubapp.ExchangeUserAuthorizationCode(r.Context(), h.githubApp.ClientID, h.githubApp.ClientSecret, code)
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": "github_user_authorization_failed"})
		return
	}
	if !h.userOwnsInstallation(r.Context(), userToken, installationID) {
		httpx.WriteJSON(w, http.StatusForbidden, map[string]string{"error": "github_installation_not_authorized_for_user"})
		return
	}
	privateKey, err := h.installationPrivateKey(r.Context(), installationID)
	if err != nil {
		httpx.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": "github_installation_verification_failed"})
		return
	}
	installationLogin, _ := githubapp.InstallationAccountLogin(r.Context(), h.githubApp.AppID, privateKey, installationID)
	now := time.Now()
	item := infra.SCMConnection{ID: h.store.NextID(), Provider: "github", Name: fmt.Sprintf("GitHub App 安装 #%d", installationID), CredentialStatus: "verified", VerifiedAt: &now, TokenConfigured: true, AuthType: "github_app", GitHubAppID: h.githubApp.AppID, InstallationID: &installationID, InstallationLogin: installationLogin}
	if err := h.store.Create(r.Context(), item, h.githubApp.PrivateKeyRef, actorID); err != nil {
		httpx.WriteJSON(w, 409, map[string]string{"error": "github_installation_already_connected"})
		return
	}
	emitSCMAudit(r.Context(), h.audit, principal, "scm.connection.created", "scm_connection", item.ID, map[string]any{
		"name": item.Name, "provider": item.Provider, "auth_type": item.AuthType, "credential_status": item.CredentialStatus,
		"github_installation_id": installationID,
	})
	if h.githubApp.FrontendURL != "" {
		http.Redirect(w, r, h.githubApp.FrontendURL+"/#/target?github_app=connected", http.StatusSeeOther)
		return
	}
	httpx.WriteJSON(w, 201, item)
}

func (h *scmHandlers) oauthCallbackParams(r *http.Request) (iam.Principal, int64, string, string, bool) {
	principal, ok := iam.PrincipalFromContext(r.Context())
	actorID := actorIDFromSubject(principal.Subject)
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if !ok || actorID <= 0 || state == "" || code == "" || !h.githubApp.configured() {
		return principal, 0, "", "", false
	}
	return principal, actorID, state, code, true
}

func (h *scmHandlers) userOwnsInstallation(ctx context.Context, userToken string, installationID int64) bool {
	ownsInstallation, err := githubapp.UserOwnsInstallation(ctx, userToken, h.githubApp.AppID, installationID)
	return err == nil && ownsInstallation
}

func (h *scmHandlers) installationPrivateKey(ctx context.Context, installationID int64) (string, error) {
	privateKey, err := readSCMSecret(ctx, h.githubApp.PrivateKeyRef)
	if err != nil {
		return "", err
	}
	if _, err := githubapp.InstallationToken(ctx, h.githubApp.AppID, privateKey, installationID); err != nil {
		return "", err
	}
	return privateKey, nil
}

func parseConnectionID(r *http.Request) (int64, bool) {
	var id int64
	if _, err := fmt.Sscan(r.PathValue("id"), &id); err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func resolveSCMConnectionToken(ctx context.Context, authType, ref, appID string, installationID *int64) (string, error) {
	if authType == "github_app" {
		if appID == "" || installationID == nil {
			return "", fmt.Errorf("github app installation missing")
		}
		privateKey, err := readSCMSecret(ctx, ref)
		if err != nil {
			return "", err
		}
		return githubapp.InstallationToken(ctx, appID, privateKey, *installationID)
	}
	return readSCMSecret(ctx, ref)
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

// actorIDFromSubject keeps numeric subjects stable and derives a deterministic
// positive ID for OIDC/WeChat subjects that are not decimal user IDs. This
// keeps authorization state and audit actors distinct without a schema change.
func actorIDFromSubject(subject string) int64 {
	subject = strings.TrimSpace(subject)
	if v, err := strconv.ParseInt(subject, 10, 64); err == nil && v > 0 {
		return v
	}
	if subject == "" {
		return 0
	}
	sum := sha256.Sum256([]byte(subject))
	v := int64(binary.BigEndian.Uint64(sum[:8]) & 0x7fffffffffffffff)
	if v == 0 {
		return 1
	}
	return v
}

func emitSCMAudit(ctx context.Context, audit SCMAuditFunc, principal iam.Principal, op, asset string, assetID int64, payload map[string]any) {
	if audit == nil {
		return
	}
	actor := actorIDFromSubject(principal.Subject)
	if actor <= 0 {
		actor, _ = rls.UserFrom(ctx)
	}
	if err := audit(ctx, actor, op, asset, assetID, payload); err != nil {
		slog.Error("scm audit append failed", "op", op, "error", err)
	}
}

func probeGitHubReadToken(ctx context.Context, token, authType string) (githubTokenInfo, error) {
	var info githubTokenInfo
	endpoint := "/user"
	if authType == "github_app" {
		endpoint = "/installation/repositories?per_page=1"
	}
	body, headers, err := githubGET(ctx, endpoint, token, 512*1024)
	if err != nil {
		return info, err
	}
	if authType != "github_app" {
		info.Scopes = splitScopes(headers.Get("X-OAuth-Scopes"))
		if len(info.Scopes) > 0 {
			return info, fmt.Errorf("classic token scopes are not allowed: %s", strings.Join(info.Scopes, ","))
		}
		var user struct {
			Login string `json:"login"`
		}
		if json.Unmarshal(body, &user) == nil {
			info.Login = user.Login
		}
	}
	return info, nil
}

func githubGET(ctx context.Context, endpoint, token string, maxBytes int64) ([]byte, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPIBase+endpoint, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	req.Header.Set("User-Agent", "greenpass-scm-connection")
	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if readErr != nil {
		return nil, resp.Header, readErr
	}
	if int64(len(body)) > maxBytes {
		return nil, resp.Header, fmt.Errorf("GitHub response exceeded the configured size limit")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.Header, fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}
	return body, resp.Header, nil
}

func splitScopes(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' })
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func writeSCMSecret(ctx context.Context, namespace, name, token string, createOnly bool) error {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of":   "greenpass",
				"app.kubernetes.io/component": "scm-credential",
			},
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{"token": token},
	}
	_, err = client.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err == nil {
		return nil
	}
	if createOnly || !apierrors.IsAlreadyExists(err) {
		return err
	}
	existing, getErr := client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if getErr != nil {
		return getErr
	}
	if existing.Data == nil {
		existing.Data = map[string][]byte{}
	}
	existing.Data["token"] = []byte(token)
	_, err = client.CoreV1().Secrets(namespace).Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

func updateSCMSecret(ctx context.Context, ref, token string) error {
	namespace, name, key, ok := parseKubernetesSecretRef(ref)
	if !ok {
		return fmt.Errorf("invalid secret ref")
	}
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}
	secret, err := client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secret.Data[key] = []byte(token)
	_, err = client.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	return err
}

func deleteSCMSecret(ctx context.Context, namespace, name string) error {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}
	err = client.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func readSCMSecret(ctx context.Context, ref string) (string, error) {
	namespace, name, key, ok := parseKubernetesSecretRef(ref)
	if !ok {
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
	secret, err := client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	value := secret.Data[key]
	if len(value) == 0 {
		return "", fmt.Errorf("secret key missing")
	}
	return string(value), nil
}

func parseKubernetesSecretRef(ref string) (namespace, name, key string, ok bool) {
	if !strings.HasPrefix(ref, "k8s://") {
		return "", "", "", false
	}
	parts := strings.Split(strings.TrimPrefix(ref, "k8s://"), "/")
	if len(parts) != 2 {
		return "", "", "", false
	}
	nameKey := strings.SplitN(parts[1], "#", 2)
	if len(nameKey) != 2 || parts[0] == "" || nameKey[0] == "" || nameKey[1] == "" {
		return "", "", "", false
	}
	return parts[0], nameKey[0], nameKey[1], true
}
