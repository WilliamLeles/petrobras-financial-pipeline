CREATE SCHEMA IF NOT EXISTS financeiro;

-- Tabela fato com granularidade mensal por conceito financeiro.
CREATE TABLE IF NOT EXISTS financeiro.petrobras_financeiro_mensal (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    competencia DATE NOT NULL,
    data_publicacao DATE NOT NULL,
    demonstrativo VARCHAR(20) NOT NULL,
    codigo_conta VARCHAR(60) NOT NULL,
    descricao_conta TEXT NOT NULL,
    valor NUMERIC(20,4) NOT NULL,
    moeda CHAR(3) NOT NULL DEFAULT 'BRL',
    escala VARCHAR(20) NOT NULL DEFAULT 'UNIDADE',
    fonte VARCHAR(120) NOT NULL,
    fonte_url TEXT,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    atualizado_em TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Dominios e integridade basica.
    CONSTRAINT ck_demonstrativo
        CHECK (demonstrativo IN ('DRE','BALANCO','DFC','DVA','NOTAS')),
    CONSTRAINT ck_moeda_iso
        CHECK (moeda ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_escala
        CHECK (escala IN ('UNIDADE','MIL','MILHAO','BILHAO')),
    CONSTRAINT ck_competencia_mes
        CHECK (competencia = date_trunc('month', competencia)::date),
    -- Permite valor negativo apenas para conceitos que naturalmente podem ter sinal negativo.
    CONSTRAINT ck_valor_sinal_financeiro
        CHECK (
            valor >= 0
            OR codigo_conta IN (
                'us-gaap:NetIncomeLoss',
                'us-gaap:OperatingIncomeLoss',
                'us-gaap:NetCashProvidedByUsedInOperatingActivities',
                'us-gaap:NetCashProvidedByUsedInInvestingActivities',
                'us-gaap:NetCashProvidedByUsedInFinancingActivities',
                'us-gaap:StockholdersEquity',
                'ifrs-full:ProfitLoss',
                'ifrs-full:CashFlowsFromUsedInOperatingActivities',
                'ifrs-full:CashFlowsFromUsedInInvestingActivities',
                'ifrs-full:CashFlowsFromUsedInFinancingActivities',
                'ifrs-full:NetCashFlowsFromUsedInOperatingActivities',
                'ifrs-full:NetCashFlowsFromUsedInInvestingActivities',
                'ifrs-full:NetCashFlowsFromUsedInFinancingActivities',
                'ifrs-full:Equity'
            )
        ),
    -- Chave de negocio para upsert idempotente do ETL.
    CONSTRAINT uq_linha_financeira
        UNIQUE (competencia, demonstrativo, codigo_conta, moeda, escala, fonte)
);

-- Indices de consulta mais frequentes (tempo, conta e data de publicacao).
CREATE INDEX IF NOT EXISTS idx_petrobras_fin_competencia
    ON financeiro.petrobras_financeiro_mensal (competencia DESC);

CREATE INDEX IF NOT EXISTS idx_petrobras_fin_conta
    ON financeiro.petrobras_financeiro_mensal (codigo_conta);

CREATE INDEX IF NOT EXISTS idx_petrobras_fin_pub
    ON financeiro.petrobras_financeiro_mensal (data_publicacao DESC);
