package etl

import "time"

// FinancialRecord is the normalized model persisted in PostgreSQL.
type FinancialRecord struct {
	Competencia    time.Time
	DataPublicacao time.Time
	Demonstrativo  string
	CodigoConta    string
	DescricaoConta string
	Valor          string
	Moeda          string
	Escala         string
	Fonte          string
	FonteURL       string
}
