package handlers

import (
	"io"
	"net/http"

	"bionicpro-auth/internal/config"
	"bionicpro-auth/internal/middleware"
)

type ReportsHandler struct {
	cfg config.Config
}

func NewReportsHandler(cfg config.Config) *ReportsHandler {
	return &ReportsHandler{cfg: cfg}
}

func (h *ReportsHandler) Proxy(w http.ResponseWriter, r *http.Request) {
	data := middleware.SessionDataFromContext(r.Context())
	if data == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, h.cfg.ReportsAPIURL+"/reports", nil)
	if err != nil {
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", "Bearer "+data.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, `{"error":"reports service unavailable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
