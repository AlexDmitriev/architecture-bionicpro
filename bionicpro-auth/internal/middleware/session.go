package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"bionicpro-auth/internal/config"
	"bionicpro-auth/internal/keycloak"
	"bionicpro-auth/internal/session"
)

type contextKey string

const SessionIDKey contextKey = "sessionID"

type SessionManager struct {
	cfg      config.Config
	store    *session.Store
	keycloak *keycloak.Client
}

func NewSessionManager(cfg config.Config, store *session.Store, kc *keycloak.Client) *SessionManager {
	return &SessionManager{cfg: cfg, store: store, keycloak: kc}
}

func (m *SessionManager) readSessionID(r *http.Request) string {
	c, err := r.Cookie(m.cfg.CookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

func (m *SessionManager) setSessionCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     m.cfg.CookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(m.cfg.SessionTTL.Seconds()),
	})
}

func (m *SessionManager) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     m.cfg.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   m.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (m *SessionManager) accessExpired(data *session.Data) bool {
	return time.Now().After(data.AccessExpiresAt.Add(-5 * time.Second))
}

func (m *SessionManager) refreshAccess(ctx context.Context, sessionID string, data *session.Data) (*session.Data, error) {
	refresh, err := m.store.GetRefreshToken(ctx, sessionID)
	if err != nil || refresh == "" {
		return nil, err
	}
	tr, err := m.keycloak.RefreshToken(refresh)
	if err != nil {
		return nil, err
	}
	accessExp := time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	refreshExp := time.Now().Add(time.Duration(tr.RefreshExpiresIn) * time.Second)
	if tr.RefreshExpiresIn == 0 {
		refreshExp = time.Now().Add(24 * time.Hour)
	}
	data.AccessToken = tr.AccessToken
	data.AccessExpiresAt = accessExp
	data.RefreshExpiresAt = refreshExp
	if err := m.store.Save(ctx, sessionID, *data, tr.RefreshToken); err != nil {
		return nil, err
	}
	return m.store.Get(ctx, sessionID)
}

// rotateSession rebinds tokens to a new session id (session fixation mitigation).
func (m *SessionManager) rotateSession(w http.ResponseWriter, r *http.Request, oldID string) (string, *session.Data, error) {
	ctx := r.Context()
	newID := uuid.NewString()
	if err := m.store.Rotate(ctx, oldID, newID); err != nil {
		return "", nil, err
	}
	m.setSessionCookie(w, newID)
	data, err := m.store.Get(ctx, newID)
	return newID, data, err
}

// EnsureSession loads session, refreshes access token if needed, rotates on protected routes.
func (m *SessionManager) EnsureSession(rotate bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionID := m.readSessionID(r)
			if sessionID == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			ctx := r.Context()
			data, err := m.store.Get(ctx, sessionID)
			if err != nil || data == nil {
				m.clearSessionCookie(w)
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			if m.accessExpired(data) {
				data, err = m.refreshAccess(ctx, sessionID, data)
				if err != nil || data == nil {
					m.clearSessionCookie(w)
					_ = m.store.Delete(ctx, sessionID)
					http.Error(w, `{"error":"session expired"}`, http.StatusUnauthorized)
					return
				}
			}

			if rotate {
				newID, newData, err := m.rotateSession(w, r, sessionID)
				if err != nil {
					http.Error(w, `{"error":"session rotation failed"}`, http.StatusInternalServerError)
					return
				}
				sessionID = newID
				data = newData
			}

			ctx = context.WithValue(ctx, SessionIDKey, sessionID)
			ctx = context.WithValue(ctx, contextKey("sessionData"), data)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (m *SessionManager) GetOptionalSession() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionID := m.readSessionID(r)
			if sessionID == "" {
				next.ServeHTTP(w, r)
				return
			}
			ctx := r.Context()
			data, err := m.store.Get(ctx, sessionID)
			if err != nil || data == nil {
				m.clearSessionCookie(w)
				next.ServeHTTP(w, r)
				return
			}
			if m.accessExpired(data) {
				data, err = m.refreshAccess(ctx, sessionID, data)
				if err != nil || data == nil {
					m.clearSessionCookie(w)
					_ = m.store.Delete(ctx, sessionID)
					next.ServeHTTP(w, r)
					return
				}
			}
			ctx = context.WithValue(ctx, SessionIDKey, sessionID)
			ctx = context.WithValue(ctx, contextKey("sessionData"), data)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func SessionDataFromContext(ctx context.Context) *session.Data {
	v, _ := ctx.Value(contextKey("sessionData")).(*session.Data)
	return v
}

func (m *SessionManager) SetCookie(w http.ResponseWriter, sessionID string) {
	m.setSessionCookie(w, sessionID)
}

func (m *SessionManager) ClearCookie(w http.ResponseWriter) {
	m.clearSessionCookie(w)
}
