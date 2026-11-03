package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/governance/infra"
	"github.com/openware-io/open-green-pass/internal/iam"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// modelProviders contains only provider defaults (vendor, protocol, base URL).
// Model IDs are never hard-coded: they are discovered from the provider's
// official GET /models endpoint after a credential is configured.
type modelProvider struct {
	Provider, Name, Protocol, BaseURL string
}

var modelProviders = []modelProvider{
	{"openai", "OpenAI", "api", "https://api.openai.com/v1"},
	{"anthropic", "Anthropic", "cc", "https://api.anthropic.com"},
	{"deepseek", "DeepSeek", "api", "https://api.deepseek.com/v1"},
	{"qwen", "Qwen", "api", "https://dashscope.aliyuncs.com/compatible-mode/v1"},
}

type providerModel struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by,omitempty"`
}

type modelDiscoverRequest struct {
	Provider string `json:"provider"`
	Protocol string `json:"protocol"`
	Token    string `json:"token"`
	BaseURL  string `json:"base_url"`
}

type modelConnectRequest struct {
	Provider  string   `json:"provider"`
	Protocol  string   `json:"protocol"`
	Token     string   `json:"token"`
	BaseURL   string   `json:"base_url"`
	ModelKeys []string `json:"model_keys"`
}

type configuredModelResponse struct {
	ID       int64  `json:"id"`
	Provider string `json:"provider"`
	ModelKey string `json:"model_key"`
	Status   string `json:"status"`
}

type modelConnectionDetailResponse struct {
	ID               int64      `json:"id"`
	Provider         string     `json:"provider"`
	ModelKey         string     `json:"model_key"`
	Protocol         string     `json:"protocol"`
	BaseURL          string     `json:"base_url"`
	CredentialStatus string     `json:"credential_status"`
	ConfiguredAt     *time.Time `json:"configured_at,omitempty"`
	TokenConfigured  bool       `json:"token_configured"`
}

//nolint:gocyclo // legacy complexity; refactor tracked in lint backlog
func RegisterModels(mux *http.ServeMux, store *infra.ModelStore, gen *id.Generator, namespace string) {
	mux.HandleFunc("GET /model-catalog", func(w http.ResponseWriter, r *http.Request) {
		models, err := store.ListModels(r.Context(), tenantID(r))
		if err != nil {
			httpx.WriteJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		configured := make([]configuredModelResponse, 0, len(models))
		for _, model := range models {
			configured = append(configured, configuredModelResponse{ID: model.ID, Provider: model.Provider, ModelKey: model.ModelKey, Status: string(model.Status)})
		}
		httpx.WriteJSON(w, 200, map[string]any{"providers": modelProviders, "models": configured})
	})
	mux.HandleFunc("GET /model-connections", func(w http.ResponseWriter, r *http.Request) {
		if !catalogEditor(w, r) {
			return
		}
		details, err := store.ListConnectionDetails(r.Context(), tenantID(r))
		if err != nil {
			httpx.WriteJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		out := make([]modelConnectionDetailResponse, 0, len(details))
		for _, item := range details {
			out = append(out, modelConnectionDetailResponse{
				ID: item.ID, Provider: item.Provider, ModelKey: item.ModelKey, Protocol: item.Protocol,
				BaseURL: item.BaseURL, CredentialStatus: item.CredentialStatus, ConfiguredAt: item.ConfiguredAt,
				TokenConfigured: item.CredentialStatus == "configured" || item.CredentialStatus == "verified",
			})
		}
		httpx.WriteJSON(w, 200, map[string]any{"connections": out})
	})
	// Discover the provider's official model list without persisting anything.
	// Token may be empty when the provider already has a stored Secret.
	mux.HandleFunc("POST /model-provider-models", func(w http.ResponseWriter, r *http.Request) {
		if !catalogEditor(w, r) {
			return
		}
		var in modelDiscoverRequest
		if httpx.DecodeJSON(r, &in) != nil {
			httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_request"})
			return
		}
		baseURL, ok := resolveProviderBaseURL(in.Provider, in.Protocol, in.BaseURL)
		if !ok {
			httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_base_url"})
			return
		}
		team := tenantID(r)
		secretName := fmt.Sprintf("gp-model-%d-%s-%s", team, safeName(in.Provider), safeName(in.Protocol))
		token, err := resolveModelToken(r.Context(), namespace, secretName, in.Token)
		if err != nil {
			httpx.WriteJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		models, err := fetchProviderModels(r.Context(), in.Protocol, baseURL, token)
		if err != nil {
			httpx.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": "provider_connection_failed"})
			return
		}
		httpx.WriteJSON(w, 200, map[string]any{"models": models})
	})
	mux.HandleFunc("POST /model-connections", func(w http.ResponseWriter, r *http.Request) {
		if !catalogEditor(w, r) {
			return
		}
		var in modelConnectRequest
		if httpx.DecodeJSON(r, &in) != nil {
			httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_request"})
			return
		}
		baseURL, ok := resolveProviderBaseURL(in.Provider, in.Protocol, in.BaseURL)
		if !ok || len(in.ModelKeys) == 0 {
			httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_model_selection"})
			return
		}
		principal, _ := iam.PrincipalFromContext(r.Context())
		team := tenantID(r)
		secretName := fmt.Sprintf("gp-model-%d-%s-%s", team, safeName(in.Provider), safeName(in.Protocol))
		token, err := resolveModelToken(r.Context(), namespace, secretName, in.Token)
		if err != nil {
			httpx.WriteJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if in.Protocol == "cc" {
			if err := probeModelConnection(r.Context(), in.Protocol, baseURL, token, in.ModelKeys[0]); err != nil {
				httpx.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": "provider_connection_failed"})
				return
			}
		} else {
			official, err := fetchProviderModels(r.Context(), in.Protocol, baseURL, token)
			if err != nil {
				httpx.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": "provider_connection_failed"})
				return
			}
			available := make(map[string]bool, len(official))
			for _, item := range official {
				available[item.ID] = true
			}
			for _, modelKey := range in.ModelKeys {
				if !available[modelKey] {
					httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_model_selection"})
					return
				}
			}
		}
		if strings.TrimSpace(in.Token) != "" {
			if err := writeModelSecret(r.Context(), namespace, secretName, in.Token); err != nil {
				httpx.WriteJSON(w, 500, map[string]string{"error": "secret_store_failed"})
				return
			}
		}
		ref := "k8s://" + namespace + "/" + secretName + "#token"
		for _, modelKey := range in.ModelKeys {
			if err := store.UpsertConnection(r.Context(), gen.Next(), team, parseSubject(principal.Subject), modelKey, in.Provider, modelKey, in.Protocol, baseURL, ref, []string{"code"}); err != nil {
				httpx.WriteJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
		}
		httpx.WriteJSON(w, 200, map[string]string{"status": "configured"})
	})
}

func resolveProviderBaseURL(provider, protocol, raw string) (string, bool) {
	if strings.TrimSpace(raw) != "" {
		return validBaseURL(raw)
	}
	for _, item := range modelProviders {
		if item.Provider == provider && item.Protocol == protocol && strings.TrimSpace(item.BaseURL) != "" {
			return item.BaseURL, true
		}
	}
	return "", false
}

func resolveModelToken(ctx context.Context, namespace, secretName, raw string) (string, error) {
	token := strings.TrimSpace(raw)
	if token != "" {
		return token, nil
	}
	// Adding a model to an already configured provider must not require the
	// operator to paste the same token again.
	existing, err := readModelSecret(ctx, namespace, secretName)
	if err != nil || strings.TrimSpace(existing) == "" {
		return "", fmt.Errorf("token_required")
	}
	return existing, nil
}

// fetchProviderModels walks the provider's official model list. CC proxies do
// not expose a model catalogue; their model is an operator-supplied name.
func fetchProviderModels(ctx context.Context, protocol, baseURL, token string) ([]providerModel, error) {
	if protocol == "cc" {
		return []providerModel{}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("provider status %d", resp.StatusCode)
	}
	var payload struct {
		Data []providerModel `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	sort.Slice(payload.Data, func(i, j int) bool { return payload.Data[i].ID < payload.Data[j].ID })
	return payload.Data, nil
}

func validBaseURL(raw string) (string, bool) {
	candidate := strings.TrimRight(strings.TrimSpace(raw), "/")
	parsed, err := url.Parse(candidate)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", false
	}
	return candidate, true
}

func probeModelConnection(ctx context.Context, protocol, baseURL, token, model string) error {
	if protocol == "cc" {
		payload := strings.NewReader(fmt.Sprintf(`{"model":%q,"max_tokens":1,"messages":[{"role":"user","content":"ping"}]}`, model))
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/messages", payload)
		if err != nil {
			return err
		}
		req.Header.Set("x-api-key", token)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("Content-Type", "application/json")
		resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("provider status %d", resp.StatusCode)
		}
		return nil
	}
	return fmt.Errorf("probe only supports cc protocol")
}

func tenantID(r *http.Request) int64 { v, _ := rls.TenantFrom(r.Context()); return v }
func parseSubject(v string) int64    { n, _ := strconv.ParseInt(v, 10, 64); return n }
func safeName(v string) string {
	return strings.Trim(strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '-'
	}, strings.ToLower(v)), "-")
}

func writeModelSecret(ctx context.Context, ns, name, token string) error {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}
	secrets := client.CoreV1().Secrets(ns)
	current, err := secrets.Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		current.StringData = map[string]string{"token": token}
		_, err = secrets.Update(ctx, current, metav1.UpdateOptions{})
		return err
	}
	_, err = secrets.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name}, Type: corev1.SecretTypeOpaque, StringData: map[string]string{"token": token}}, metav1.CreateOptions{})
	return err
}

func readModelSecret(ctx context.Context, ns, name string) (string, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return "", err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return "", err
	}
	secret, err := client.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return string(secret.Data["token"]), nil
}
