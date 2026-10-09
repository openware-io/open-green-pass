package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/iam"
	iaminfra "github.com/openware-io/open-green-pass/internal/iam/infra"
)

type wechatConfigInput struct {
	Enabled     bool   `json:"enabled"`
	AppID       string `json:"app_id"`
	AppSecret   string `json:"app_secret"`
	CallbackURI string `json:"callback_uri"`
}
type wechatToken struct {
	AccessToken string `json:"access_token"`
	OpenID      string `json:"openid"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}
type wechatProfile struct {
	OpenID     string `json:"openid"`
	UnionID    string `json:"unionid"`
	Nickname   string `json:"nickname"`
	HeadImgURL string `json:"headimgurl"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

func RegisterWechat(mux *http.ServeMux, store *iaminfra.WechatStore, auth *iaminfra.AuthStore, namespace string, secureCookie bool) {
	mux.HandleFunc("GET /auth/wechat/status", func(w http.ResponseWriter, r *http.Request) {
		teamID, _ := strconv.ParseInt(r.URL.Query().Get("team_id"), 10, 64)
		cfg, err := store.PublicConfig(r.Context(), teamID)
		if err == pgx.ErrNoRows {
			httpx.WriteJSON(w, 200, map[string]any{"configured": false, "enabled": false})
			return
		}
		if err != nil {
			httpx.WriteJSON(w, 500, map[string]string{"error": "config_read_failed"})
			return
		}
		httpx.WriteJSON(w, 200, map[string]any{"configured": cfg.AppID != "" && cfg.CallbackURI != "" && cfg.SecretRef != "", "enabled": cfg.Enabled})
	})
	mux.HandleFunc("GET /settings/auth/wechat", func(w http.ResponseWriter, r *http.Request) {
		teamID, _, ok := mutationIdentity(w, r)
		if !ok {
			return
		}
		cfg, err := store.Config(r.Context(), teamID)
		if err == pgx.ErrNoRows {
			httpx.WriteJSON(w, 200, map[string]any{"team_id": teamID, "enabled": false, "app_id": "", "callback_uri": "", "secret_configured": false})
			return
		}
		if err != nil {
			writeIAM(w, nil, err)
			return
		}
		httpx.WriteJSON(w, 200, map[string]any{"team_id": cfg.TeamID, "enabled": cfg.Enabled, "app_id": cfg.AppID, "callback_uri": cfg.CallbackURI, "secret_configured": cfg.SecretRef != "", "updated_at": cfg.UpdatedAt})
	})
	mux.HandleFunc("PUT /settings/auth/wechat", func(w http.ResponseWriter, r *http.Request) {
		teamID, actorID, ok := mutationIdentity(w, r)
		if !ok {
			return
		}
		var in wechatConfigInput
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&in) != nil || strings.TrimSpace(in.AppID) == "" || strings.TrimSpace(in.CallbackURI) == "" {
			httpx.WriteJSON(w, 400, map[string]string{"error": "app_id_and_callback_required"})
			return
		}
		callback, err := url.ParseRequestURI(in.CallbackURI)
		if err != nil || (callback.Scheme != "https" && callback.Hostname() != "greenpass.localhost") || callback.Hostname() == "" {
			httpx.WriteJSON(w, 400, map[string]string{"error": "https_callback_required"})
			return
		}
		secretName := fmt.Sprintf("gp-auth-wechat-%d", teamID)
		secretRef := "k8s://" + namespace + "/" + secretName + "#app-secret"
		current, _ := store.Config(r.Context(), teamID)
		if strings.TrimSpace(in.AppSecret) != "" {
			if err = writeWechatSecret(r.Context(), namespace, secretName, in.AppSecret); err != nil {
				httpx.WriteJSON(w, 500, map[string]string{"error": "secret_store_failed"})
				return
			}
		} else if current.SecretRef == "" {
			httpx.WriteJSON(w, 400, map[string]string{"error": "app_secret_required"})
			return
		}
		if current.SecretRef != "" {
			secretRef = current.SecretRef
		}
		cfg := iaminfra.WechatConfig{TeamID: teamID, Enabled: in.Enabled, AppID: strings.TrimSpace(in.AppID), CallbackURI: strings.TrimSpace(in.CallbackURI), SecretRef: secretRef}
		if err = store.SaveConfig(r.Context(), cfg, actorID); err != nil {
			writeIAM(w, nil, err)
			return
		}
		httpx.WriteJSON(w, 200, map[string]any{"status": "configured", "secret_configured": true})
	})
	mux.HandleFunc("GET /auth/wechat/start", func(w http.ResponseWriter, r *http.Request) {
		teamID, _ := strconv.ParseInt(r.URL.Query().Get("team_id"), 10, 64)
		startWechat(w, r, store, teamID, 0, "login")
	})
	mux.HandleFunc("GET /account/wechat/start", func(w http.ResponseWriter, r *http.Request) {
		teamID, userID, ok := requestIdentity(w, r)
		if !ok {
			return
		}
		startWechat(w, r, store, teamID, userID, "bind")
	})
	mux.HandleFunc("GET /account/wechat", func(w http.ResponseWriter, r *http.Request) {
		_, userID, ok := requestIdentity(w, r)
		if !ok {
			return
		}
		identity, err := store.IdentityForUser(r.Context(), userID)
		if err == pgx.ErrNoRows {
			httpx.WriteJSON(w, 200, map[string]any{"bound": false})
			return
		}
		if err != nil {
			writeIAM(w, nil, err)
			return
		}
		httpx.WriteJSON(w, 200, map[string]any{"bound": true, "display_name": identity.DisplayName, "avatar_url": identity.AvatarURL})
	})
	mux.HandleFunc("DELETE /account/wechat", func(w http.ResponseWriter, r *http.Request) {
		_, userID, ok := requestIdentity(w, r)
		if !ok {
			return
		}
		err := store.Unbind(r.Context(), userID)
		if err == pgx.ErrNoRows {
			w.WriteHeader(204)
			return
		}
		if err != nil {
			writeIAM(w, nil, err)
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("GET /auth/wechat/callback", func(w http.ResponseWriter, r *http.Request) {
		state, err := store.ConsumeState(r.Context(), r.URL.Query().Get("state"))
		if err != nil {
			redirectWechatError(w, r, "invalid_state")
			return
		}
		cfg, err := store.PublicConfig(r.Context(), state.TeamID)
		if err != nil || !cfg.Enabled {
			redirectWechatError(w, r, "not_configured")
			return
		}
		secret, err := readWechatSecret(r.Context(), cfg.SecretRef)
		if err != nil {
			redirectWechatError(w, r, "secret_unavailable")
			return
		}
		profile, err := wechatExchange(r.Context(), cfg.AppID, secret, r.URL.Query().Get("code"))
		if err != nil {
			redirectWechatError(w, r, "provider_exchange_failed")
			return
		}
		if state.Purpose == "bind" {
			profile.UserID = state.UserID
			if err = store.Bind(r.Context(), profile); err != nil {
				redirectWechatError(w, r, "bind_failed")
				return
			}
			http.Redirect(w, r, "/#/settings?wechat=bound", http.StatusFound)
			return
		}
		userID, err := store.UserForIdentity(r.Context(), profile.OpenID)
		if err != nil {
			redirectWechatError(w, r, "account_not_bound")
			return
		}
		_, token, expires, err := auth.CreateSession(r.Context(), userID, state.TeamID)
		if err != nil {
			redirectWechatError(w, r, "session_failed")
			return
		}
		http.SetCookie(w, &http.Cookie{Name: iaminfra.SessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: secureCookie, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: int(time.Until(expires).Seconds())})
		http.Redirect(w, r, "/#/scenarios", http.StatusFound)
	})
}

func startWechat(w http.ResponseWriter, r *http.Request, store *iaminfra.WechatStore, teamID, userID int64, purpose string) {
	cfg, err := store.PublicConfig(r.Context(), teamID)
	if err != nil || !cfg.Enabled || cfg.AppID == "" || cfg.SecretRef == "" {
		httpx.WriteJSON(w, 409, map[string]string{"error": "wechat_not_configured"})
		return
	}
	state, err := store.NewState(r.Context(), teamID, userID, purpose)
	if err != nil {
		httpx.WriteJSON(w, 500, map[string]string{"error": "state_create_failed"})
		return
	}
	query := url.Values{"appid": {cfg.AppID}, "redirect_uri": {cfg.CallbackURI}, "response_type": {"code"}, "scope": {"snsapi_login"}, "state": {state}}
	http.Redirect(w, r, "https://open.weixin.qq.com/connect/qrconnect?"+query.Encode()+"#wechat_redirect", http.StatusFound)
}
func wechatExchange(ctx context.Context, appID, secret, code string) (iaminfra.WechatIdentity, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	tokenURL := "https://api.weixin.qq.com/sns/oauth2/access_token?" + url.Values{"appid": {appID}, "secret": {secret}, "code": {code}, "grant_type": {"authorization_code"}}.Encode()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	resp, err := client.Do(req)
	if err != nil {
		return iaminfra.WechatIdentity{}, err
	}
	defer resp.Body.Close()
	var token wechatToken
	if json.NewDecoder(resp.Body).Decode(&token) != nil || token.ErrCode != 0 || token.AccessToken == "" {
		return iaminfra.WechatIdentity{}, fmt.Errorf("wechat token: %s", token.ErrMsg)
	}
	profileURL := "https://api.weixin.qq.com/sns/userinfo?" + url.Values{"access_token": {token.AccessToken}, "openid": {token.OpenID}, "lang": {"zh_CN"}}.Encode()
	req, _ = http.NewRequestWithContext(ctx, http.MethodGet, profileURL, nil)
	resp, err = client.Do(req)
	if err != nil {
		return iaminfra.WechatIdentity{}, err
	}
	defer resp.Body.Close()
	var profile wechatProfile
	if json.NewDecoder(resp.Body).Decode(&profile) != nil || profile.ErrCode != 0 || profile.OpenID == "" {
		return iaminfra.WechatIdentity{}, fmt.Errorf("wechat profile: %s", profile.ErrMsg)
	}
	return iaminfra.WechatIdentity{OpenID: profile.OpenID, UnionID: profile.UnionID, DisplayName: profile.Nickname, AvatarURL: profile.HeadImgURL}, nil
}
func redirectWechatError(w http.ResponseWriter, r *http.Request, code string) {
	http.Redirect(w, r, "/#/login?wechat_error="+url.QueryEscape(code), http.StatusFound)
}
func writeWechatSecret(ctx context.Context, ns, name, value string) error {
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
		current.StringData = map[string]string{"app-secret": value}
		_, err = secrets.Update(ctx, current, metav1.UpdateOptions{})
		return err
	}
	_, err = secrets.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name}, Type: corev1.SecretTypeOpaque, StringData: map[string]string{"app-secret": value}}, metav1.CreateOptions{})
	return err
}
func readWechatSecret(ctx context.Context, ref string) (string, error) {
	parts := strings.Split(strings.TrimPrefix(ref, "k8s://"), "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid secret ref")
	}
	nameKey := strings.Split(parts[1], "#")
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
	value := string(secret.Data[nameKey[1]])
	if value == "" {
		return "", fmt.Errorf("empty secret")
	}
	return value, nil
}

var _ = iam.ErrForbidden
