package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"bionicpro-reports/internal/keycloak"
	"bionicpro-reports/internal/report"
	"bionicpro-reports/internal/storage"
)

type ReportsHandler struct {
	kc      *keycloak.Client
	repo    *report.Repository
	storage *storage.S3
}

func NewReportsHandler(kc *keycloak.Client, repo *report.Repository, reportStorage *storage.S3) *ReportsHandler {
	return &ReportsHandler{kc: kc, repo: repo, storage: reportStorage}
}

func (h *ReportsHandler) Get(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	ui, err := h.kc.FetchUserInfo(accessToken)
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	userID := ui.Email
	if userID == "" {
		userID = ui.PreferredUsername
	}
	if userID == "" {
		http.Error(w, `{"error":"subject_not_found_in_token"}`, http.StatusUnauthorized)
		return
	}

	// API не должен отдавать чужие отчеты: если клиент передал user_id,
	// он обязан совпадать с субъектом из токена.
	requestedUserID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if requestedUserID != "" && requestedUserID != userID {
		http.Error(w, `{"error":"forbidden_user_scope"}`, http.StatusForbidden)
		return
	}

	key := h.storage.ObjectKey(userID)
	fromCache, err := h.storage.Exists(r.Context(), key)
	if err != nil {
		http.Error(w, `{"error":"failed_to_read_s3_cache"}`, http.StatusInternalServerError)
		return
	}

	if !fromCache {
		payload, repoErr := h.repo.GetPayloadByUserID(r.Context(), userID)
		if repoErr != nil {
			if errors.Is(repoErr, pgx.ErrNoRows) {
				http.Error(w, `{"error":"report_not_found"}`, http.StatusNotFound)
				return
			}
			http.Error(w, `{"error":"failed_to_read_report"}`, http.StatusInternalServerError)
			return
		}

		if err := h.storage.SaveReport(r.Context(), key, payload); err != nil {
			http.Error(w, `{"error":"failed_to_save_report_to_s3"}`, http.StatusInternalServerError)
			return
		}
	}

	resp := map[string]any{
		"report_url": h.storage.CDNURL(key),
		"cached":     fromCache,
	}
	body, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, `{"error":"failed_to_encode_response"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func bearerToken(authorization string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(authorization, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authorization, prefix))
	if token == "" {
		return "", false
	}
	return token, true
}
