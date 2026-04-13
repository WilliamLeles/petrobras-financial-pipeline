-- Evolução mensal por demonstrativo e moeda
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
