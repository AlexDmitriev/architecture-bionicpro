package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"bionicpro-auth/internal/config"
	"bionicpro-auth/internal/keycloak"
	"bionicpro-auth/internal/middleware"
	"bionicpro-auth/internal/profile"
	"bionicpro-auth/internal/session"
	"bionicpro-auth/internal/yandex"
)

const yandexDirectIdPHint = "yandex_direct"

type AuthHandler struct {
	cfg      config.Config
	store    *session.Store
	kc       *keycloak.Client
	sessMgr  *middleware.SessionManager
	profiles *profile.Repository
}

func NewAuthHandler(
	cfg config.Config,
	store *session.Store,
	kc *keycloak.Client,
	sessMgr *middleware.SessionManager,
	profiles *profile.Repository,
) *AuthHandler {
	return &AuthHandler{cfg: cfg, store: store, kc: kc, sessMgr: sessMgr, profiles: profiles}
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

func (h *AuthHandler) startLogin(w http.ResponseWriter, r *http.Request, idpHint string) {
	verifier, challenge, err := generatePKCE()
	if err != nil {
		http.Error(w, "failed to generate PKCE", http.StatusInternalServerError)
		return
	}
	state := uuid.NewString()
	if err := h.store.SaveOAuthState(r.Context(), state, session.OAuthState{
		Verifier: verifier,
		IdpHint:  idpHint,
	}); err != nil {
		http.Error(w, "failed to save state", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, h.kc.BuildAuthURL(state, challenge, idpHint), http.StatusFound)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	h.startLogin(w, r, "")
}

func (h *AuthHandler) LoginYandex(w http.ResponseWriter, r *http.Request) {
	verifier, challenge, err := generatePKCE()
	if err != nil {
		http.Error(w, "failed to generate PKCE", http.StatusInternalServerError)
		return
	}
	state := uuid.NewString()
	if err := h.store.SaveOAuthState(r.Context(), state, session.OAuthState{
		Verifier: verifier,
		IdpHint:  yandexDirectIdPHint,
	}); err != nil {
		http.Error(w, "failed to save state", http.StatusInternalServerError)
		return
	}

	yc := yandex.NewClient()
	redirectURL := yc.BuildAuthURL(h.cfg.YandexClientID, h.cfg.YandexRedirectURI, state, challenge)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		http.Error(w, "missing code or state", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	oauthState, err := h.store.GetOAuthState(ctx, state)
	if err != nil || oauthState == nil || oauthState.Verifier == "" {
		http.Error(w, "invalid or expired state", http.StatusBadRequest)
		return
	}
	_ = h.store.DeleteOAuthState(ctx, state)

	if oauthState.IdpHint == yandexDirectIdPHint {
		h.callbackYandexDirect(w, r, code, oauthState.Verifier)
		return
	}

	tr, err := h.kc.ExchangeCode(code, oauthState.Verifier)
	if err != nil {
		http.Error(w, "token exchange failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	ui, err := h.kc.FetchUserInfo(tr.AccessToken)
	if err != nil {
		http.Error(w, "userinfo failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	idp := oauthState.IdpHint
	if idp == "" {
		idp = ui.IdentityProvider
	}

	sessionID := uuid.NewString()
	accessExp := time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	refreshExp := time.Now().Add(time.Duration(tr.RefreshExpiresIn) * time.Second)
	if tr.RefreshExpiresIn == 0 {
		refreshExp = time.Now().Add(24 * time.Hour)
	}

	sessData := session.Data{
		AccessToken:      tr.AccessToken,
		AccessExpiresAt:  accessExp,
		RefreshExpiresAt: refreshExp,
		KeycloakSub:      ui.Sub,
		IdentityProvider: idp,
	}
	if err := h.store.Save(ctx, sessionID, sessData, tr.RefreshToken); err != nil {
		http.Error(w, "failed to save session", http.StatusInternalServerError)
		return
	}

	h.sessMgr.SetCookie(w, sessionID)

	if idp == "yandex" && h.profiles != nil {
		hasConsent, err := h.profiles.HasConsent(ctx, ui.Sub)
		if err == nil && !hasConsent {
			http.Redirect(w, r, h.cfg.FrontendURL+"/consent", http.StatusFound)
			return
		}
	}

	http.Redirect(w, r, h.cfg.FrontendURL, http.StatusFound)
}

func (h *AuthHandler) callbackYandexDirect(w http.ResponseWriter, r *http.Request, code, codeVerifier string) {
	if h.cfg.YandexClientID == "" || h.cfg.YandexClientSecret == "" {
		http.Error(w, "yandex credentials are not configured", http.StatusInternalServerError)
		return
	}

	yc := yandex.NewClient()
	yandexToken, expiresIn, err := yc.ExchangeCode(
		h.cfg.YandexClientID,
		h.cfg.YandexClientSecret,
		h.cfg.YandexRedirectURI,
		code,
		codeVerifier,
	)
	if err != nil {
		http.Error(w, "yandex token exchange failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	yp, _, err := yc.FetchProfile(yandexToken)
	if err != nil {
		http.Error(w, "failed to fetch yandex profile: "+err.Error(), http.StatusBadGateway)
		return
	}

	username := yp.Login
	if username == "" {
		username = strings.SplitN(yp.DefaultEmail, "@", 2)[0]
	}
	provisionUsername := "yandex_" + username
	keycloakSub, keycloakUsername, err := h.kc.EnsureUser(yp.DefaultEmail, provisionUsername, yp.FirstName, yp.LastName)
	if err != nil {
		http.Error(w, "failed to provision user in keycloak: "+err.Error(), http.StatusBadGateway)
		return
	}

	servicePassword, err := keycloak.GenerateServicePassword()
	if err != nil {
		http.Error(w, "failed to generate service password", http.StatusInternalServerError)
		return
	}
	if err := h.kc.SetUserPassword(keycloakSub, servicePassword); err != nil {
		http.Error(w, "failed to set keycloak password: "+err.Error(), http.StatusBadGateway)
		return
	}
	tr, err := h.kc.ExchangePassword(keycloakUsername, servicePassword)
	if err != nil {
		http.Error(w, "failed to exchange keycloak token: "+err.Error(), http.StatusBadGateway)
		return
	}

	if expiresIn <= 0 && tr.ExpiresIn > 0 {
		expiresIn = tr.ExpiresIn
	}
	if expiresIn <= 0 {
		expiresIn = 300
	}
	sessionID := uuid.NewString()
	accessExp := time.Now().Add(time.Duration(expiresIn) * time.Second)
	refreshExp := time.Now().Add(24 * time.Hour)
	if tr.RefreshExpiresIn > 0 {
		refreshExp = time.Now().Add(time.Duration(tr.RefreshExpiresIn) * time.Second)
	}
	sessData := session.Data{
		AccessToken:      tr.AccessToken,
		AccessExpiresAt:  accessExp,
		RefreshExpiresAt: refreshExp,
		KeycloakSub:      keycloakSub,
		IdentityProvider: "yandex",
	}
	if err := h.store.Save(r.Context(), sessionID, sessData, tr.RefreshToken); err != nil {
		http.Error(w, "failed to save session", http.StatusInternalServerError)
		return
	}

	if h.profiles != nil {
		rec := profile.Record{
			KeycloakSub: keycloakSub,
			YandexID:    yp.ID,
			Login:       yp.Login,
			Email:       yp.DefaultEmail,
			FirstName:   yp.FirstName,
			LastName:    yp.LastName,
			DisplayName: yp.DisplayName,
			AvatarURL:   yp.AvatarURL(),
		}
		if err := h.profiles.UpsertWithoutConsent(r.Context(), rec); err != nil {
			http.Error(w, "failed to save profile pre-consent", http.StatusInternalServerError)
			return
		}
	}

	h.sessMgr.SetCookie(w, sessionID)
	if h.profiles != nil {
		hasConsent, err := h.profiles.HasConsent(r.Context(), keycloakSub)
		if err == nil && !hasConsent {
			http.Redirect(w, r, h.cfg.FrontendURL+"/consent", http.StatusFound)
			return
		}
	}
	http.Redirect(w, r, h.cfg.FrontendURL, http.StatusFound)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	data := middleware.SessionDataFromContext(r.Context())
	if data == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"authenticated": false})
		return
	}

	resp := map[string]interface{}{
		"authenticated":     true,
		"access_expires_at": data.AccessExpiresAt.UTC().Format(time.RFC3339),
		"identity_provider": data.IdentityProvider,
		"needs_consent":     false,
	}

	if data.IdentityProvider == "yandex" && h.profiles != nil && data.KeycloakSub != "" {
		hasConsent, err := h.profiles.HasConsent(r.Context(), data.KeycloakSub)
		if err == nil {
			resp["needs_consent"] = !hasConsent
			resp["consent_granted"] = hasConsent
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
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
