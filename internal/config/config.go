package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	// HTTP server port.
	Port string
	// PostgreSQL DSN in URL format.
	DatabaseURL string
	// Required by SEC fair access policy.
	SecUserAgent string
	// Timeout for outbound SEC requests.
	SecTimeout time.Duration
	// Initial ETL time window in months.
	DefaultMonths int
	// Minimum target records before stopping fallback expansion.
	MinRecords int
	// Maximum fallback time window in months.
	MaxFallbackMonths int
	// SEC Company Facts endpoint.
	CompanyFactsURL string
}

// Load reads environment variables and validates runtime configuration.
func Load() (Config, error) {
	port := envOrDefault("PORT", "8080")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL obrigatoria")
	}

	secUserAgent := os.Getenv("SEC_USER_AGENT")
	if secUserAgent == "" {
		return Config{}, fmt.Errorf("SEC_USER_AGENT obrigatoria (ex: MinhaAPI/1.0 contato@empresa.com)")
	}

	secTimeoutSeconds, err := intFromEnv("SEC_TIMEOUT_SECONDS", 30)
	if err != nil {
		return Config{}, err
	}

	defaultMonths, err := intFromEnv("ETL_DEFAULT_MONTHS", 6)
	if err != nil {
		return Config{}, err
	}

	minRecords, err := intFromEnv("ETL_MIN_RECORDS", 50)
	if err != nil {
		return Config{}, err
	}

	maxFallbackMonths, err := intFromEnv("ETL_MAX_FALLBACK_MONTHS", 120)
	if err != nil {
		return Config{}, err
	}
	if maxFallbackMonths < defaultMonths {
		maxFallbackMonths = defaultMonths
	}

	companyFactsURL := envOrDefault("SEC_COMPANY_FACTS_URL", "https://data.sec.gov/api/xbrl/companyfacts/CIK0001119639.json")

	return Config{
		Port:              port,
		DatabaseURL:       databaseURL,
		SecUserAgent:      secUserAgent,
		SecTimeout:        time.Duration(secTimeoutSeconds) * time.Second,
		DefaultMonths:     defaultMonths,
		MinRecords:        minRecords,
		MaxFallbackMonths: maxFallbackMonths,
		CompanyFactsURL:   companyFactsURL,
	}, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func intFromEnv(key string, fallback int) (int, error) {
	v := envOrDefault(key, strconv.Itoa(fallback))
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s invalido: %w", key, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s deve ser > 0", key)
	}
	return n, nil
}
