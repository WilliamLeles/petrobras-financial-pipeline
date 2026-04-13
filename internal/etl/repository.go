package etl

import (
	"context"
	"database/sql"
	"fmt"
)

type Repository struct {
	db *sql.DB
}

// NewRepository creates a persistence layer for ETL records.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// UpsertBatch writes normalized records using the table unique key.
func (r *Repository) UpsertBatch(ctx context.Context, records []FinancialRecord) (inserted int, err error) {
	if len(records) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("erro ao abrir transacao: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	const q = `
		INSERT INTO financeiro.petrobras_financeiro_mensal (
			competencia,
			data_publicacao,
			demonstrativo,
			codigo_conta,
			descricao_conta,
			valor,
			moeda,
			escala,
			fonte,
			fonte_url,
			atualizado_em
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW())
		ON CONFLICT (competencia, demonstrativo, codigo_conta, moeda, escala, fonte)
		DO UPDATE SET
			data_publicacao = EXCLUDED.data_publicacao,
			descricao_conta = EXCLUDED.descricao_conta,
			valor = EXCLUDED.valor,
			fonte_url = EXCLUDED.fonte_url,
			atualizado_em = NOW()
		WHERE financeiro.petrobras_financeiro_mensal.data_publicacao <= EXCLUDED.data_publicacao
	`

	stmt, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return 0, fmt.Errorf("erro ao preparar upsert: %w", err)
	}
	defer stmt.Close()

	for _, rec := range records {
		if _, err = stmt.ExecContext(
			ctx,
			rec.Competencia,
			rec.DataPublicacao,
			rec.Demonstrativo,
			rec.CodigoConta,
			rec.DescricaoConta,
			rec.Valor,
			rec.Moeda,
			rec.Escala,
			rec.Fonte,
			rec.FonteURL,
		); err != nil {
			return 0, fmt.Errorf("erro ao executar upsert: %w", err)
		}
		inserted++
	}

	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("erro ao commitar transacao: %w", err)
	}

	return inserted, nil
}

// ListRecent returns the most recent records for API inspection.
func (r *Repository) ListRecent(ctx context.Context, limit int) ([]FinancialRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	const q = `
		SELECT competencia, data_publicacao, demonstrativo, codigo_conta, descricao_conta, valor::text, moeda, escala, fonte, COALESCE(fonte_url, '')
		FROM financeiro.petrobras_financeiro_mensal
		ORDER BY competencia DESC, demonstrativo, codigo_conta
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar registros: %w", err)
	}
	defer rows.Close()

	out := make([]FinancialRecord, 0, limit)
	for rows.Next() {
		var rec FinancialRecord
		if err := rows.Scan(
			&rec.Competencia,
			&rec.DataPublicacao,
			&rec.Demonstrativo,
			&rec.CodigoConta,
			&rec.DescricaoConta,
			&rec.Valor,
			&rec.Moeda,
			&rec.Escala,
			&rec.Fonte,
			&rec.FonteURL,
		); err != nil {
			return nil, fmt.Errorf("erro ao ler registro: %w", err)
		}
		out = append(out, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erro na iteracao de registros: %w", err)
	}

	return out, nil
}
