package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"bionicpro-auth/internal/config"
	"bionicpro-auth/internal/keycloak"
	"bionicpro-auth/internal/middleware"
	"bionicpro-auth/internal/session"
)

type AuthHandler struct {
	cfg     config.Config
	store   *session.Store
	kc      *keycloak.Client
	sessMgr *middleware.SessionManager
}

func NewAuthHandler(cfg config.Config, store *session.Store, kc *keycloak.Client, sessMgr *middleware.SessionManager) *AuthHandler {
	return &AuthHandler{cfg: cfg, store: store, kc: kc, sessMgr: sessMgr}
}

func generatePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	verifier = base64URLEncode(b)
	h := sha256.Sum256([]byte(verifier))
	challenge = base64URLEncode(h[:])
	return verifier, challenge, nil
}

func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	verifier, challenge, err := generatePKCE()
	if err != nil {
		http.Error(w, "failed to generate PKCE", http.StatusInternalServerError)
		return
	}
	state := uuid.NewString()
	if err := h.store.SavePKCE(r.Context(), state, verifier); err != nil {
		http.Error(w, "failed to save state", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, h.kc.BuildAuthURL(state, challenge), http.StatusFound)
}

func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		http.Error(w, "missing code or state", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	verifier, err := h.store.GetPKCE(ctx, state)
	if err != nil || verifier == "" {
		http.Error(w, "invalid or expired state", http.StatusBadRequest)
		return
	}
	_ = h.store.DeletePKCE(ctx, state)

	tr, err := h.kc.ExchangeCode(code, verifier)
	if err != nil {
		http.Error(w, "token exchange failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	sessionID := uuid.NewString()
	accessExp := time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	refreshExp := time.Now().Add(time.Duration(tr.RefreshExpiresIn) * time.Second)
	if tr.RefreshExpiresIn == 0 {
		refreshExp = time.Now().Add(24 * time.Hour)
	}
	if err := h.store.Save(ctx, sessionID, tr.AccessToken, tr.RefreshToken, accessExp, refreshExp); err != nil {
		http.Error(w, "failed to save session", http.StatusInternalServerError)
		return
	}

	h.sessMgr.SetCookie(w, sessionID)
	http.Redirect(w, r, h.cfg.FrontendURL, http.StatusFound)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	data := middleware.SessionDataFromContext(r.Context())
	if data == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"authenticated": false})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"authenticated":     true,
		"access_expires_at": data.AccessExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	sessionID := ""
	if c, err := r.Cookie(h.cfg.CookieName); err == nil {
		sessionID = c.Value
	}
	if sessionID != "" {
		if refresh, err := h.store.GetRefreshToken(r.Context(), sessionID); err == nil && refresh != "" {
			_ = h.kc.Logout(refresh)
		}
		_ = h.store.Delete(r.Context(), sessionID)
	}
	h.sessMgr.ClearCookie(w)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (h *AuthHandler) AccessToken(w http.ResponseWriter, r *http.Request) {
	data := middleware.SessionDataFromContext(r.Context())
	if data == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"access_token": data.AccessToken,
	})
}
