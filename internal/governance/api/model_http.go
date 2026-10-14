package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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

type modelPreset struct {
	Provider, Name, ModelKey, Protocol, BaseURL string
	Capabilities                                []string
}

var modelPresets = []modelPreset{
	{"openai", "GPT-6 Astra", "gpt-6-astra", "api", "https://api.openai.com/v1", []string{"reasoning", "code", "vision"}},
	{"openai", "GPT-5.6 Terra", "gpt-5.6-terra", "api", "https://api.openai.com/v1", []string{"reasoning", "code", "vision"}},
	{"openai", "GPT-5.6 Luna", "gpt-5.6-luna", "api", "https://api.openai.com/v1", []string{"code", "vision"}},
	{"anthropic", "Claude Opus 5.5", "claude-opus-5-5", "cc", "https://api.anthropic.com", []string{"reasoning", "code", "vision"}},
	{"deepseek", "DeepSeek V4 Pro", "deepseek-v4-pro", "api", "https://api.deepseek.com/v1", []string{"reasoning", "code", "vision"}},
	{"deepseek", "DeepSeek V4.1 Flash", "deepseek-v4.1-flash", "api", "https://api.deepseek.com/v1", []string{"reasoning", "code"}},
	{"qwen", "Qwen3.8 Max", "qwen3.8-max", "api", "https://dashscope.aliyuncs.com/compatible-mode/v1", []string{"reasoning", "code", "vision"}},
	{"qwen", "Qwen3.8 Flash", "qwen3.8-flash", "api", "https://dashscope.aliyuncs.com/compatible-mode/v1", []string{"code", "vision"}},
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
		httpx.WriteJSON(w, 200, map[string]any{"presets": modelPresets, "models": configured})
	})
	mux.HandleFunc("POST /model-connections", func(w http.ResponseWriter, r *http.Request) {
		if !catalogEditor(w, r) {
			return
		}
		var in modelConnectRequest
		if httpx.DecodeJSON(r, &in) != nil || strings.TrimSpace(in.Token) == "" {
			httpx.WriteJSON(w, 400, map[string]string{"error": "token_required"})
			return
		}
		principal, _ := iam.PrincipalFromContext(r.Context())
		team := tenantID(r)
		baseURL, ok := selectedBaseURL(in)
		if !ok || len(in.ModelKeys) == 0 {
			httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_model_selection"})
			return
		}
		if err := probeModelConnection(r.Context(), in.Protocol, baseURL, in.Token, in.ModelKeys[0]); err != nil {
			httpx.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": "provider_connection_failed"})
			return
		}
		secretName := fmt.Sprintf("gp-model-%d-%s-%s", team, safeName(in.Provider), safeName(in.Protocol))
		if err := writeModelSecret(r.Context(), namespace, secretName, in.Token); err != nil {
			httpx.WriteJSON(w, 500, map[string]string{"error": "secret_store_failed"})
			return
		}
		for _, modelKey := range in.ModelKeys {
			preset, ok := findPreset(in.Provider, in.Protocol, modelKey)
			if !ok {
				if in.Protocol != "cc" {
					continue
				}
				preset = modelPreset{Provider: in.Provider, Name: modelKey, ModelKey: modelKey, Protocol: "cc", BaseURL: baseURL, Capabilities: []string{"code"}}
			}
			ref := "k8s://" + namespace + "/" + secretName + "#token"
			if err := store.UpsertConnection(r.Context(), gen.Next(), team, parseSubject(principal.Subject), preset.Name, preset.Provider, preset.ModelKey, preset.Protocol, baseURL, ref, preset.Capabilities); err != nil {
				httpx.WriteJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
		}
		httpx.WriteJSON(w, 200, map[string]string{"status": "configured"})
	})
}
func findPreset(provider, protocol, key string) (modelPreset, bool) {
	for _, preset := range modelPresets {
		if preset.Provider == provider && preset.Protocol == protocol && preset.ModelKey == key {
			return preset, true
		}
	}
	return modelPreset{}, false
}
func selectedBaseURL(in modelConnectRequest) (string, bool) {
	if in.Protocol == "cc" && strings.TrimSpace(in.BaseURL) != "" && len(in.ModelKeys) > 0 {
		candidate := strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
		parsed, err := url.Parse(candidate)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return "", false
		}
		return candidate, true
	}
	for _, preset := range modelPresets {
		if preset.Provider == in.Provider && preset.Protocol == in.Protocol && contains(in.ModelKeys, preset.ModelKey) {
			if strings.TrimSpace(in.BaseURL) != "" {
				return strings.TrimRight(in.BaseURL, "/"), true
			}
			return preset.BaseURL, true
		}
	}
	return "", false
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/models", nil)
	if err != nil {
		return err
	}
	if protocol == "cc" {
		req.Header.Set("x-api-key", token)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("provider status %d", resp.StatusCode)
	}
	return nil
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
func contains(values []string, v string) bool {
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
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
