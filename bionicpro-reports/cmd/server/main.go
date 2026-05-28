package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"bionicpro-reports/internal/config"
	"bionicpro-reports/internal/handlers"
	"bionicpro-reports/internal/keycloak"
	"bionicpro-reports/internal/report"
	"bionicpro-reports/internal/storage"
)

func main() {
	cfg := config.Load()

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("postgres ping: %v", err)
	}

	kc := keycloak.NewClient(cfg)
	repo := report.NewRepository(pool)
	reportStorage, err := storage.NewS3(cfg)
	if err != nil {
		log.Fatalf("s3 init: %v", err)
	}
	if err := reportStorage.EnsureBucket(context.Background()); err != nil {
		log.Fatalf("s3 bucket init: %v", err)
	}
	reportsH := handlers.NewReportsHandler(kc, repo, reportStorage)

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/reports", reportsH.Get)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("bionicpro-reports listening on %s", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
