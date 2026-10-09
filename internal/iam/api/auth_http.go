package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/iam"
	iaminfra "github.com/openware-io/open-green-pass/internal/iam/infra"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func RegisterAuth(mux *http.ServeMux, store *iaminfra.AuthStore, secureCookie bool) {
	mux.HandleFunc("POST /auth/login", func(w http.ResponseWriter, r *http.Request) {
		var in loginRequest
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in) != nil || strings.TrimSpace(in.Username) == "" || in.Password == "" {
			httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_request"})
			return
		}
		u, token, expires, err := store.Login(r.Context(), in.Username, in.Password)
		if err != nil {
			time.Sleep(150 * time.Millisecond)
			httpx.WriteJSON(w, 401, map[string]string{"error": "invalid_credentials"})
			return
		}
		http.SetCookie(w, &http.Cookie{Name: iaminfra.SessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: secureCookie, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: int(time.Until(expires).Seconds())})
		httpx.WriteJSON(w, 200, map[string]any{"user": u, "expires_at": expires})
	})
	mux.HandleFunc("GET /auth/me", func(w http.ResponseWriter, r *http.Request) {
		p, ok := iam.PrincipalFromContext(r.Context())
		if !ok {
			httpx.WriteJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		u, err := store.Current(r.Context(), p)
		if err != nil {
			httpx.WriteJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		httpx.WriteJSON(w, 200, map[string]any{"user": u})
	})
	mux.HandleFunc("POST /auth/logout", func(w http.ResponseWriter, r *http.Request) {
		c, _ := r.Cookie(iaminfra.SessionCookie)
		if c != nil {
			_ = store.Logout(r.Context(), c.Value)
		}
		http.SetCookie(w, &http.Cookie{Name: iaminfra.SessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: secureCookie, SameSite: http.SameSiteLaxMode, MaxAge: -1})
		w.WriteHeader(http.StatusNoContent)
	})
}
