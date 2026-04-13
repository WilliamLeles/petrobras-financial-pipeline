package etl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

type secClient struct {
	httpClient *http.Client
	userAgent  string
	url        string
}

type companyFactsResponse struct {
	Facts map[string]map[string]factItem `json:"facts"`
}

type factItem struct {
	Label string                       `json:"label"`
	Units map[string][]factObservation `json:"units"`
}

type factObservation struct {
	End   string      `json:"end"`
	Filed string      `json:"filed"`
	Form  string      `json:"form"`
	Val   json.Number `json:"val"`
}

func newSECClient(timeout time.Duration, userAgent, url string) *secClient {
	return &secClient{
		httpClient: &http.Client{Timeout: timeout},
		userAgent:  userAgent,
		url:        url,
	}
}

// NewSECClient creates a client for SEC Company Facts.
func NewSECClient(timeout time.Duration, userAgent, url string) *secClient {
	return newSECClient(timeout, userAgent, url)
}

// fetchRecords downloads and normalizes financial facts by months window.
func (c *secClient) fetchRecords(ctx context.Context, months int) ([]FinancialRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao montar request sec: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar dados da sec: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("sec retornou status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()

	var payload companyFactsResponse
	if err := dec.Decode(&payload); err != nil {
		return nil, fmt.Errorf("erro ao decodificar json sec: %w", err)
	}

	cutoff := firstDayOfMonth(time.Now().AddDate(0, -(months - 1), 0))
	m := make(map[string]FinancialRecord)

	for _, concept := range selectedConcepts {
		factsByNamespace, ok := payload.Facts[concept.Namespace]
		if !ok {
			continue
		}

		fact, ok := factsByNamespace[concept.Tag]
		if !ok {
			continue
		}

		unit := pickUnit(fact.Units)
		if unit == "" {
			continue
		}

		for _, obs := range fact.Units[unit] {
			if obs.Filed == "" {
				continue
			}

			filedDate, err := parseSECDate(obs.Filed)
			if err != nil {
				continue
			}

			competencia := filedDate
			if obs.End != "" {
				if endDate, err := parseSECDate(obs.End); err == nil {
					competencia = endDate
				}
			}

			// Accept facts with recent competence OR recent filing date.
			if firstDayOfMonth(competencia).Before(cutoff) && firstDayOfMonth(filedDate).Before(cutoff) {
				continue
			}

			valor := obs.Val.String()
			if valor == "" {
				continue
			}

			rec := FinancialRecord{
				Competencia:    firstDayOfMonth(competencia),
				DataPublicacao: filedDate,
				Demonstrativo:  concept.Demonstrativo,
				CodigoConta:    concept.Namespace + ":" + concept.Tag,
				DescricaoConta: fallbackLabel(fact.Label, concept.Descricao),
				Valor:          valor,
				Moeda:          mapUnitToCurrency(unit),
				Escala:         "UNIDADE",
				Fonte:          "SEC_EDGAR",
				FonteURL:       c.url,
			}

			key := uniqueKey(rec)
			if current, exists := m[key]; !exists || rec.DataPublicacao.After(current.DataPublicacao) {
				m[key] = rec
			}
		}
	}

	records := make([]FinancialRecord, 0, len(m))
	for _, r := range m {
		records = append(records, r)
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].Competencia.Equal(records[j].Competencia) {
			if records[i].Demonstrativo == records[j].Demonstrativo {
				return records[i].CodigoConta < records[j].CodigoConta
			}
			return records[i].Demonstrativo < records[j].Demonstrativo
		}
		return records[i].Competencia.Before(records[j].Competencia)
	})

	return records, nil
}

func parseSECDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("data vazia")
	}

	layouts := []string{
		"2006-01-02",
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC(), nil
		}
	}

	if len(raw) >= 10 {
		if t, err := time.Parse("2006-01-02", raw[:10]); err == nil {
			return t.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("formato de data invalido: %s", raw)
}

// uniqueKey mirrors the database unique constraint fields.
func uniqueKey(r FinancialRecord) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s", r.Competencia.Format("2006-01-02"), r.Demonstrativo, r.CodigoConta, r.Moeda, r.Escala, r.Fonte)
}

// pickUnit prioritizes currency units commonly used by Petrobras filings.
func pickUnit(units map[string][]factObservation) string {
	order := []string{"USD", "BRL"}
	for _, u := range order {
		if _, ok := units[u]; ok {
			return u
		}
	}
	for u := range units {
		if u == "USD/shares" || strings.HasPrefix(u, "shares") {
			continue
		}
		return u
	}
	return ""
}

func fallbackLabel(label, fallback string) string {
	if strings.TrimSpace(label) != "" {
		return label
	}
	return fallback
}

// mapUnitToCurrency converts SEC unit labels to ISO currency codes.
func mapUnitToCurrency(unit string) string {
	u := strings.ToUpper(unit)
	if strings.Contains(u, "BRL") {
		return "BRL"
	}
	if strings.Contains(u, "USD") {
		return "USD"
	}
	return "USD"
}

func firstDayOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

type conceptMeta struct {
	Namespace     string
	Tag           string
	Demonstrativo string
	Descricao     string
}

// selectedConcepts defines supported concepts across US GAAP and IFRS taxonomies.
var selectedConcepts = []conceptMeta{
	// US GAAP
	{Namespace: "us-gaap", Tag: "Revenues", Demonstrativo: "DRE", Descricao: "Receitas"},
	{Namespace: "us-gaap", Tag: "CostsAndExpenses", Demonstrativo: "DRE", Descricao: "Custos e Despesas"},
	{Namespace: "us-gaap", Tag: "OperatingIncomeLoss", Demonstrativo: "DRE", Descricao: "Resultado Operacional"},
	{Namespace: "us-gaap", Tag: "NetIncomeLoss", Demonstrativo: "DRE", Descricao: "Lucro Liquido"},
	{Namespace: "us-gaap", Tag: "Assets", Demonstrativo: "BALANCO", Descricao: "Ativos"},
	{Namespace: "us-gaap", Tag: "Liabilities", Demonstrativo: "BALANCO", Descricao: "Passivos"},
	{Namespace: "us-gaap", Tag: "StockholdersEquity", Demonstrativo: "BALANCO", Descricao: "Patrimonio Liquido"},
	{Namespace: "us-gaap", Tag: "CashAndCashEquivalentsAtCarryingValue", Demonstrativo: "BALANCO", Descricao: "Caixa e Equivalentes"},
	{Namespace: "us-gaap", Tag: "NetCashProvidedByUsedInOperatingActivities", Demonstrativo: "DFC", Descricao: "Fluxo de Caixa Operacional"},
	{Namespace: "us-gaap", Tag: "NetCashProvidedByUsedInInvestingActivities", Demonstrativo: "DFC", Descricao: "Fluxo de Caixa de Investimentos"},
	{Namespace: "us-gaap", Tag: "NetCashProvidedByUsedInFinancingActivities", Demonstrativo: "DFC", Descricao: "Fluxo de Caixa de Financiamento"},

	// IFRS (comum para foreign issuers como Petrobras)
	{Namespace: "ifrs-full", Tag: "Revenue", Demonstrativo: "DRE", Descricao: "Receitas"},
	{Namespace: "ifrs-full", Tag: "RevenueFromContractsWithCustomers", Demonstrativo: "DRE", Descricao: "Receitas de Contratos com Clientes"},
	{Namespace: "ifrs-full", Tag: "ProfitLoss", Demonstrativo: "DRE", Descricao: "Lucro Liquido"},
	{Namespace: "ifrs-full", Tag: "Assets", Demonstrativo: "BALANCO", Descricao: "Ativos"},
	{Namespace: "ifrs-full", Tag: "Liabilities", Demonstrativo: "BALANCO", Descricao: "Passivos"},
	{Namespace: "ifrs-full", Tag: "Equity", Demonstrativo: "BALANCO", Descricao: "Patrimonio Liquido"},
	{Namespace: "ifrs-full", Tag: "CashAndCashEquivalents", Demonstrativo: "BALANCO", Descricao: "Caixa e Equivalentes"},
	{Namespace: "ifrs-full", Tag: "CashFlowsFromUsedInOperatingActivities", Demonstrativo: "DFC", Descricao: "Fluxo de Caixa Operacional"},
	{Namespace: "ifrs-full", Tag: "CashFlowsFromUsedInInvestingActivities", Demonstrativo: "DFC", Descricao: "Fluxo de Caixa de Investimentos"},
	{Namespace: "ifrs-full", Tag: "CashFlowsFromUsedInFinancingActivities", Demonstrativo: "DFC", Descricao: "Fluxo de Caixa de Financiamento"},
	{Namespace: "ifrs-full", Tag: "NetCashFlowsFromUsedInOperatingActivities", Demonstrativo: "DFC", Descricao: "Fluxo de Caixa Operacional"},
	{Namespace: "ifrs-full", Tag: "NetCashFlowsFromUsedInInvestingActivities", Demonstrativo: "DFC", Descricao: "Fluxo de Caixa de Investimentos"},
	{Namespace: "ifrs-full", Tag: "NetCashFlowsFromUsedInFinancingActivities", Demonstrativo: "DFC", Descricao: "Fluxo de Caixa de Financiamento"},
}
