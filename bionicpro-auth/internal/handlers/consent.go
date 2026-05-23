package handlers

import (
	"encoding/json"
	"net/http"

	"bionicpro-auth/internal/config"
	"bionicpro-auth/internal/keycloak"
	"bionicpro-auth/internal/middleware"
	"bionicpro-auth/internal/profile"
	"bionicpro-auth/internal/yandex"
)

type ConsentHandler struct {
	cfg      config.Config
	profiles *profile.Repository
	kc       *keycloak.Client
	yandex   *yandex.Client
}

func NewConsentHandler(cfg config.Config, profiles *profile.Repository, kc *keycloak.Client) *ConsentHandler {
	return &ConsentHandler{
		cfg:      cfg,
		profiles: profiles,
		kc:       kc,
		yandex:   yandex.NewClient(),
	}
}

func (h *ConsentHandler) Status(w http.ResponseWriter, r *http.Request) {
	data := middleware.SessionDataFromContext(r.Context())
	if data == nil || data.KeycloakSub == "" {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	needsConsent := data.IdentityProvider == "yandex"
	hasConsent := false
	if needsConsent {
		var err error
		hasConsent, err = h.profiles.HasConsent(r.Context(), data.KeycloakSub)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"identity_provider": data.IdentityProvider,
		"needs_consent":     needsConsent && !hasConsent,
		"consent_granted":   hasConsent,
	})
}

func (h *ConsentHandler) Accept(w http.ResponseWriter, r *http.Request) {
	data := middleware.SessionDataFromContext(r.Context())
	if data == nil || data.KeycloakSub == "" {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if data.IdentityProvider != "yandex" {
		http.Error(w, `{"error":"consent not required"}`, http.StatusBadRequest)
		return
	}

	rec, err := h.fetchAndBuildProfile(data.AccessToken, data.KeycloakSub)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch yandex profile"}`, http.StatusBadGateway)
		return
	}

	if err := h.profiles.UpsertWithConsent(r.Context(), *rec); err != nil {
		http.Error(w, `{"error":"failed to save profile"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"profile": mapProfileResponse(rec),
	})
}

func (h *ConsentHandler) Profile(w http.ResponseWriter, r *http.Request) {
	data := middleware.SessionDataFromContext(r.Context())
	if data == nil || data.KeycloakSub == "" {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	rec, err := h.profiles.GetBySub(r.Context(), data.KeycloakSub)
	if err != nil {
		http.Error(w, `{"error":"profile not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(mapProfileResponse(rec))
}

func (h *ConsentHandler) fetchAndBuildProfile(keycloakAccessToken, keycloakSub string) (*profile.Record, error) {
	yandexToken, err := h.kc.ExchangeForIdpToken(keycloakAccessToken, "yandex")
	if err != nil {
		ui, uerr := h.kc.FetchUserInfo(keycloakAccessToken)
		if uerr != nil {
			return nil, err
		}
		raw, _ := json.Marshal(ui)
		return &profile.Record{
			KeycloakSub: keycloakSub,
			Email:       ui.Email,
			FirstName:   ui.GivenName,
			LastName:    ui.FamilyName,
			DisplayName: ui.Name,
			Login:       ui.PreferredUsername,
			RawProfile:  raw,
		}, nil
	}

	yp, raw, err := h.yandex.FetchProfile(yandexToken)
	if err != nil {
		return nil, err
	}

	return &profile.Record{
		KeycloakSub:  keycloakSub,
		YandexID:     yp.ID,
		Login:        yp.Login,
		Email:        yp.DefaultEmail,
		FirstName:    yp.FirstName,
		LastName:     yp.LastName,
		DisplayName:  yp.DisplayName,
		AvatarURL:    yp.AvatarURL(),
		RawProfile:   raw,
	}, nil
}

func mapProfileResponse(rec *profile.Record) map[string]interface{} {
	return map[string]interface{}{
		"yandex_id":    rec.YandexID,
		"login":        rec.Login,
		"email":        rec.Email,
		"first_name":   rec.FirstName,
		"last_name":    rec.LastName,
		"display_name": rec.DisplayName,
		"avatar_url":   rec.AvatarURL,
		"has_consent":  rec.ConsentGrantedAt != nil,
	}
}
