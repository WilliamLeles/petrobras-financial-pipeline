package main

import (
	"log"
	"net/http"
	"time"

	"projeto_petrobras/internal/config"
	"projeto_petrobras/internal/db"
	"projeto_petrobras/internal/etl"
	"projeto_petrobras/internal/httpapi"
)

func main() {
	// Load runtime configuration from environment variables.
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("erro de configuracao: %v", err)
	}

	// Initialize PostgreSQL connection pool.
	pg, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("erro no banco: %v", err)
	}
	defer pg.Close()

	// Wire dependencies.
	repo := etl.NewRepository(pg)
	secClient := etl.NewSECClient(cfg.SecTimeout, cfg.SecUserAgent, cfg.CompanyFactsURL)
	service := etl.NewService(secClient, repo, cfg.MinRecords, cfg.MaxFallbackMonths)

	// Expose HTTP API handlers.
	api := httpapi.New(service, repo, cfg.DefaultMonths, cfg.MinRecords, cfg.MaxFallbackMonths)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("API iniciada em http://localhost:%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("erro no servidor http: %v", err)
	}
}
