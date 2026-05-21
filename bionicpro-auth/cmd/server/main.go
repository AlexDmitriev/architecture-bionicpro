package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"

	"bionicpro-auth/internal/config"
	"bionicpro-auth/internal/handlers"
	"bionicpro-auth/internal/keycloak"
	"bionicpro-auth/internal/session"
	sessmw "bionicpro-auth/internal/middleware"
)

func corsMiddleware(frontendURL string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == frontendURL {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func main() {
	cfg := config.Load()

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("redis: %v", err)
	}

	store := session.NewStore(rdb, cfg.EncryptionKey, cfg.SessionTTL)
	kc := keycloak.NewClient(cfg)
	sessMgr := sessmw.NewSessionManager(cfg, store, kc)
	authH := handlers.NewAuthHandler(cfg, store, kc, sessMgr)
	reportsH := handlers.NewReportsHandler(cfg)

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer)
	r.Use(corsMiddleware(cfg.FrontendURL))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Route("/auth", func(ar chi.Router) {
		ar.Get("/login", authH.Login)
		ar.Get("/callback", authH.Callback)
		ar.Post("/logout", authH.Logout)

		ar.Group(func(pr chi.Router) {
			pr.Use(sessMgr.GetOptionalSession())
			pr.Get("/me", authH.Me)
		})

		ar.Group(func(pr chi.Router) {
			pr.Use(sessMgr.EnsureSession(true))
			pr.Get("/reports", reportsH.Proxy)
			pr.Get("/token", authH.AccessToken)
		})
	})

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("bionicpro-auth listening on %s", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
