package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"projeto_petrobras/internal/etl"
)

type Server struct {
	mux               *http.ServeMux
	etlService        *etl.Service
	repo              *etl.Repository
	defaultMonths     int
	defaultMinRecords int
	defaultMaxMonths  int
}

// New configures the HTTP handlers and runtime defaults.
func New(etlService *etl.Service, repo *etl.Repository, defaultMonths, defaultMinRecords, defaultMaxMonths int) *Server {
	s := &Server{
		mux:               http.NewServeMux(),
		etlService:        etlService,
		repo:              repo,
		defaultMonths:     defaultMonths,
		defaultMinRecords: defaultMinRecords,
		defaultMaxMonths:  defaultMaxMonths,
	}
	s.routes()
	return s
}

// Handler returns the root mux used by the HTTP server.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	// Health and ETL endpoints.
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/etl/run", s.handleRunETL)
	s.mux.HandleFunc("/etl/records", s.handleListRecords)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "petrobras-financeiro-etl",
		"time":    time.Now().UTC(),
	})
}

// handleRunETL triggers the ETL flow.
func (s *Server) handleRunETL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "use POST"})
		return
	}

	// Query parameter overrides for ETL controls.
	months := s.defaultMonths
	if raw := r.URL.Query().Get("months"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 240 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "months deve ser inteiro entre 1 e 240"})
			return
		}
		months = parsed
	}

	minRecords := s.defaultMinRecords
	if raw := r.URL.Query().Get("min_records"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 100000 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "min_records deve ser inteiro entre 1 e 100000"})
			return
		}
		minRecords = parsed
	}

	maxMonths := s.defaultMaxMonths
	if raw := r.URL.Query().Get("max_months"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 240 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "max_months deve ser inteiro entre 1 e 240"})
			return
		}
		maxMonths = parsed
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	result, err := s.etlService.Run(ctx, months, minRecords, maxMonths)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleListRecords(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "use GET"})
		return
	}

	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "limit deve ser inteiro > 0"})
			return
		}
		limit = parsed
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	records, err := s.repo.ListRecent(ctx, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"count": len(records),
		"items": records,
	})
}

// writeJSON standardizes JSON responses across endpoints.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
