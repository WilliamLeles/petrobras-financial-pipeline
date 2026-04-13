# Petrobras Financial ETL API

API REST em Go para ingestao de dados financeiros publicos da Petrobras (SEC EDGAR), com carga incremental em PostgreSQL via `upsert`.

## Objetivo

Este projeto foi criado para disponibilizar uma base financeira historica e consultavel para analise em BI (ex.: Metabase), com foco em:

- ingestao automatizada de dados publicos;
- modelo relacional consistente;
- idempotencia na carga;
- facilidade de operacao em ambiente local ou servidor.

## Arquitetura

Fluxo simplificado:

1. API recebe `POST /etl/run`.
2. ETL consulta `companyfacts` da SEC para Petrobras.
3. Dados sao normalizados para um modelo unico (`FinancialRecord`).
4. Carga no PostgreSQL com `ON CONFLICT DO UPDATE`.
5. Consulta operacional via `GET /etl/records`.

Camadas:

- `internal/httpapi`: endpoints HTTP e validacao de parametros;
- `internal/etl`: coleta, normalizacao e orquestracao da carga;
- `internal/db`: conexao e pool PostgreSQL;
- `schema.sql`: estrutura da base.

## Fonte de dados

Fonte publica utilizada:

- SEC EDGAR XBRL Company Facts
- URL padrao: `https://data.sec.gov/api/xbrl/companyfacts/CIK0001119639.json`

Observacao: por politica de acesso da SEC, `SEC_USER_AGENT` e obrigatorio.

## Requisitos

- Go 1.26+
- PostgreSQL 13+
- Acesso de rede para `data.sec.gov`

## Setup rapido

### 1) Criar tabela

```bash
psql "$DATABASE_URL" -f schema.sql
```

### 2) Configurar variaveis de ambiente

```bash
export DATABASE_URL='postgres://SEU_USUARIO:SUA_SENHA@localhost:5432/financeiro_petrobras?sslmode=disable'
export SEC_USER_AGENT='SeuApp/1.0 seu-email@dominio.com'
export PORT=8080
```

### 3) Subir API

```bash
go run ./cmd/server
```

## Variaveis de ambiente

| Variavel | Obrigatoria | Padrao | Descricao |
|---|---|---|---|
| `DATABASE_URL` | Sim | - | String de conexao do PostgreSQL |
| `SEC_USER_AGENT` | Sim | - | Identificacao para requests na SEC |
| `PORT` | Nao | `8080` | Porta HTTP da API |
| `SEC_TIMEOUT_SECONDS` | Nao | `30` | Timeout para chamadas externas |
| `ETL_DEFAULT_MONTHS` | Nao | `6` | Janela inicial da coleta |
| `ETL_MIN_RECORDS` | Nao | `50` | Volume minimo alvo de registros |
| `ETL_MAX_FALLBACK_MONTHS` | Nao | `120` | Limite maximo da janela de fallback |
| `SEC_COMPANY_FACTS_URL` | Nao | URL da Petrobras | Permite sobrescrever endpoint |

## Endpoints

### `GET /health`

Verifica saude da API.

Exemplo:

```bash
curl http://localhost:8080/health
```

### `POST /etl/run`

Executa ETL e retorna resumo da carga.

Query params:

- `months` (1..240): janela inicial;
- `min_records` (1..100000): alvo minimo de volume;
- `max_months` (1..240): teto da expansao da janela.

Exemplos:

```bash
curl -X POST http://localhost:8080/etl/run
curl -X POST 'http://localhost:8080/etl/run?months=6'
curl -X POST 'http://localhost:8080/etl/run?months=6&min_records=80&max_months=120'
```

Resposta (resumo):

- `fetched_records`: registros coletados da fonte;
- `upserted_records`: registros processados no banco;
- `final_months_window`: janela final usada apos fallback;
- `attempts`: quantas tentativas de expansao foram feitas.

### `GET /etl/records`

Lista registros recentes ja persistidos.

Exemplo:

```bash
curl 'http://localhost:8080/etl/records?limit=100'
```

## Estrutura de dados

Tabela principal:

- `financeiro.petrobras_financeiro_mensal`

Pontos-chave:

- `valor` em `NUMERIC(20,4)`;
- chave de negocio unica para `upsert` idempotente:
  - `(competencia, demonstrativo, codigo_conta, moeda, escala, fonte)`;
- constraints para consistencia de dominio;
- indices para consultas temporais e por conta.

## Estrategia de fallback de volume

Quando a janela inicial nao atinge `min_records`, o ETL amplia progressivamente a janela de tempo:

- `months -> months*2 -> ...`,
- ate atingir `min_records` ou `max_months`.

Essa estrategia ajuda a manter volume util para dashboards e exploracao analitica.

## Operacao recomendada (Metabase)

- Agendar `POST /etl/run` diariamente (cron/job);
- Usar `GET /etl/records` para verificacao rapida de ingestao;
- Criar views agregadas por `competencia`, `demonstrativo` e `codigo_conta` para dashboards.

## Seguranca e publicacao no GitHub

Antes de publicar:

- nao versionar credenciais (ex.: `.vscode/settings.json` com senha);
- manter `DATABASE_URL` e segredos apenas em variaveis de ambiente;
- rotacionar senha se houver qualquer exposicao acidental.

## Troubleshooting

### `DATABASE_URL obrigatoria`

A variavel nao foi exportada no shell atual.

### `role "..." nao existe`

Usuario informado na URL nao existe no PostgreSQL.

### `fetched_records: 0`

A janela pode estar curta para os conceitos disponiveis. Aumente `months` ou ajuste `min_records`/`max_months`.

## Licenca

Defina a licenca desejada antes da publicacao (ex.: MIT, Apache-2.0).
