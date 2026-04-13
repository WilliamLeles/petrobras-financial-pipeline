package etl

import (
	"context"
	"fmt"
	"time"
)

type Service struct {
	client            *secClient
	repo              *Repository
	defaultMinRecords int
	defaultMaxMonths  int
}

// RunResult summarizes a single ETL execution.
type RunResult struct {
	Months            int       `json:"months"`
	Source            string    `json:"source"`
	FetchedRecords    int       `json:"fetched_records"`
	UpsertedRecords   int       `json:"upserted_records"`
	StartedAt         time.Time `json:"started_at"`
	FinishedAt        time.Time `json:"finished_at"`
	MinRecordsTarget  int       `json:"min_records_target"`
	MaxFallbackMonths int       `json:"max_fallback_months"`
	FinalMonthsWindow int       `json:"final_months_window"`
	Attempts          int       `json:"attempts"`
}

// NewService builds an ETL service with fallback defaults.
func NewService(client *secClient, repo *Repository, defaultMinRecords, defaultMaxMonths int) *Service {
	if defaultMinRecords <= 0 {
		defaultMinRecords = 1
	}
	if defaultMaxMonths <= 0 {
		defaultMaxMonths = 24
	}
	return &Service{
		client:            client,
		repo:              repo,
		defaultMinRecords: defaultMinRecords,
		defaultMaxMonths:  defaultMaxMonths,
	}
}

// Run executes ETL with optional fallback expansion of the months window.
func (s *Service) Run(ctx context.Context, months, minRecords, maxMonths int) (RunResult, error) {
	if months <= 0 {
		months = 6
	}
	if minRecords <= 0 {
		minRecords = s.defaultMinRecords
	}
	if maxMonths <= 0 {
		maxMonths = s.defaultMaxMonths
	}
	if maxMonths < months {
		maxMonths = months
	}

	started := time.Now().UTC()

	currentWindow := months
	attempts := 0
	var records []FinancialRecord
	for {
		attempts++
		fetched, err := s.client.fetchRecords(ctx, currentWindow)
		if err != nil {
			return RunResult{}, fmt.Errorf("erro ao coletar dados publicos: %w", err)
		}
		records = fetched

		if len(records) >= minRecords || currentWindow >= maxMonths {
			break
		}

		// Expand the query window progressively until target volume is reached.
		nextWindow := currentWindow * 2
		if nextWindow > maxMonths {
			nextWindow = maxMonths
		}
		if nextWindow == currentWindow {
			break
		}
		currentWindow = nextWindow
	}

	upserted, err := s.repo.UpsertBatch(ctx, records)
	if err != nil {
		return RunResult{}, fmt.Errorf("erro ao gravar dados no postgres: %w", err)
	}

	return RunResult{
		Months:            months,
		Source:            "SEC_EDGAR/companyfacts",
		FetchedRecords:    len(records),
		UpsertedRecords:   upserted,
		StartedAt:         started,
		FinishedAt:        time.Now().UTC(),
		MinRecordsTarget:  minRecords,
		MaxFallbackMonths: maxMonths,
		FinalMonthsWindow: currentWindow,
		Attempts:          attempts,
	}, nil
}
