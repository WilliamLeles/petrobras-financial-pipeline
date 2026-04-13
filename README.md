# Petrobras Financial ETL Pipeline (Go + PostgreSQL)

Pipeline ETL em Go com endpoints HTTP operacionais para carga e inspeção de dados financeiros públicos da Petrobras (SEC EDGAR), persistidos em PostgreSQL com `upsert`.

## O que este projeto é

Este repositório é, principalmente, um **pipeline ETL**.
A camada HTTP funciona como interface operacional para:

- disparar a carga (`POST /etl/run`),
- validar saúde do serviço (`GET /health`),
- inspecionar dados persistidos (`GET /etl/records`).

## Arquitetura

```text
SEC EDGAR (companyfacts JSON)
            |
            v
   Coleta + Normalização (ETL)
            |
            v
 PostgreSQL (financeiro.petrobras_financeiro_mensal)
            |
            +--> GET /etl/records
            +--> BI (Metabase)
```

Camadas do código:

- `internal/etl`: coleta, normalização e regra de fallback;
- `internal/db`: conexão com PostgreSQL;
- `internal/httpapi`: endpoints operacionais;
- `cmd/server`: bootstrap da aplicação;
- `schema.sql`: modelo relacional.

## Fonte de dados

- SEC EDGAR XBRL Company Facts
- URL padrão: `https://data.sec.gov/api/xbrl/companyfacts/CIK0001119639.json`

`SEC_USER_AGENT` é obrigatório para aderência à política de acesso da SEC.

## Requisitos

- Go **1.26.1** (alinhado com `go.mod`)
- PostgreSQL 13+
- Acesso de rede para `data.sec.gov`

## Como testar em 3 minutos

1. Configure variáveis de ambiente:

```bash
cp .env.example .env
# ajuste os valores conforme seu ambiente
export DATABASE_URL='postgres://SEU_USUARIO:SUA_SENHA@localhost:5432/financeiro_petrobras?sslmode=disable'
export SEC_USER_AGENT='SeuApp/1.0 seu-email@dominio.com'
export PORT=8080
```

2. Crie a tabela:

```bash
psql "$DATABASE_URL" -f schema.sql
```

3. Suba a aplicação:

```bash
go run ./cmd/server
```

4. Execute ETL e consulte:

```bash
curl -X POST 'http://localhost:8080/etl/run?months=6&min_records=80&max_months=120'
curl 'http://localhost:8080/etl/records?limit=20'
```

## Variáveis de ambiente

| Variável | Obrigatória | Padrão | Descrição |
|---|---|---|---|
| `DATABASE_URL` | Sim | - | String de conexão PostgreSQL |
| `SEC_USER_AGENT` | Sim | - | Identificação para requests na SEC |
| `PORT` | Não | `8080` | Porta HTTP da API |
| `SEC_TIMEOUT_SECONDS` | Não | `30` | Timeout de request externo |
| `ETL_DEFAULT_MONTHS` | Não | `6` | Janela inicial da coleta |
| `ETL_MIN_RECORDS` | Não | `50` | Volume mínimo alvo |
| `ETL_MAX_FALLBACK_MONTHS` | Não | `120` | Teto da expansão da janela |
| `SEC_COMPANY_FACTS_URL` | Não | URL da Petrobras | Sobrescrever endpoint da SEC |

## Endpoints operacionais

### `GET /health`

```bash
curl http://localhost:8080/health
```

### `POST /etl/run`

Parâmetros:

- `months` (1..240)
- `min_records` (1..100000)
- `max_months` (1..240)

Exemplo:

```bash
curl -X POST 'http://localhost:8080/etl/run?months=6&min_records=80&max_months=120'
```

Exemplo real de resposta (execução local):

```json
{
  "months": 6,
  "source": "SEC_EDGAR/companyfacts",
  "fetched_records": 85,
  "upserted_records": 85,
  "started_at": "2026-04-08T19:30:47.140230357Z",
  "finished_at": "2026-04-08T19:30:49.122113085Z",
  "min_records_target": 80,
  "max_fallback_months": 120,
  "final_months_window": 48,
  "attempts": 4
}
```

Arquivo de referência: [`samples/run_response.json`](samples/run_response.json)

### `GET /etl/records`

```bash
curl 'http://localhost:8080/etl/records?limit=20'
```

Exemplo de resposta:

```json
{
  "count": 3,
  "items": [
    {
      "Competencia": "2025-12-01T00:00:00Z",
      "DataPublicacao": "2026-02-26T00:00:00Z",
      "Demonstrativo": "DRE",
      "CodigoConta": "ifrs-full:Revenue",
      "DescricaoConta": "Revenue",
      "Valor": "120345000000.0000",
      "Moeda": "USD",
      "Escala": "UNIDADE",
      "Fonte": "SEC_EDGAR",
      "FonteURL": "https://data.sec.gov/api/xbrl/companyfacts/CIK0001119639.json"
    }
  ]
}
```

Arquivo de referência: [`samples/records_sample.json`](samples/records_sample.json)

## Query SQL de análise pronta

```sql
SELECT
  competencia,
  demonstrativo,
  moeda,
  COUNT(*) AS qtd_contas,
  SUM(valor) AS valor_total,
  AVG(valor) AS valor_medio
FROM financeiro.petrobras_financeiro_mensal
GROUP BY competencia, demonstrativo, moeda
ORDER BY competencia DESC, demonstrativo, moeda;
```

Arquivos de referência:

- [`samples/analysis_query.sql`](samples/analysis_query.sql)
- [`samples/query_result.csv`](samples/query_result.csv)

## Modelo de dados

Tabela principal:

- `financeiro.petrobras_financeiro_mensal`

Pontos-chave:

- `valor` em `NUMERIC(20,4)` (sem `FLOAT`);
- chave única de negócio para carga idempotente;
- constraints de domínio para consistência;
- índices para leitura temporal e por conta.

## Estratégia de fallback de volume

Se a janela inicial não atingir `min_records`, o ETL amplia progressivamente:

- `months -> months*2 -> ...`,
- até atingir `min_records` ou `max_months`.

Isso melhora previsibilidade de volume para uso em BI/Metabase.

## Segurança para publicação no GitHub

- Não versionar credenciais (ex.: `.vscode/settings.json`, `.env` real).
- Use `.env.example` como template seguro.
- Se alguma senha tiver sido exposta, faça rotação imediatamente.

## Licença

Este projeto está licenciado sob MIT. Veja [`LICENSE`](LICENSE).
