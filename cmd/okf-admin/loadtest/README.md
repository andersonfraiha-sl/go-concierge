# Testes de carga do okf-admin (k6)

Scripts versionados para desempenho e resiliência, usados com `grafana/k6` via Docker
(nunca contra produção; sempre contra um okf-admin local, com `DATABASE_URL` apontando
para um Postgres seu e todas as integrações externas — Gruvi, Nexus, ATENDE, MCP — vazias).

## Como rodar

Suba o servidor numa porta sua (não 8090/8099) com um Postgres local:

```sh
DATABASE_URL="postgres://concierge:concierge@127.0.0.1:<sua-porta>/concierge?sslmode=disable" \
AMBIENTE=dev AUTH_MODE=dev AUTH_DEV_USUARIO=admin-superlogica PEDIDOS=off \
./okf-admin -root ./okf -addr 127.0.0.1:18092 -env /dev/null
```

E rode um script (de dentro desta pasta; `host.docker.internal` alcança o host a partir do
container):

```sh
docker run --rm -v "$PWD:/scripts" -e BASE_URL=http://host.docker.internal:18092 \
  grafana/k6:0.54.0 run /scripts/load.js
```

## Scripts

- `smoke.js` — checagem rápida das rotas principais (1 VU, poucas iterações).
- `load.js` — carga sustentada no pico esperado (20 VUs, padrão `VUS`/`DURATION`): catálogo,
  skill, árvore do Avançado e PUT de arquivo (idempotente). Mede p50/p95/p99/p99.9.
- `stress.js` — rampa de VUs (1→120) até achar o ponto de ruptura por latência.
- `soak.js` — carga baixa e constante por um período longo (padrão `SOAK_DURATION`).
- `concorrencia_arquivo.js` — N VUs gravando o mesmo arquivo ao mesmo tempo; confira o
  resultado com um GET depois (deve ser um dos conteúdos enviados, nunca uma mistura).
- `dependencia_fora.js` — carga leve e constante para rodar enquanto você derruba e devolve
  o Postgres por fora (ver relatório de desempenho e resiliência).
- `coldstart.sh` — mede o tempo do binário até responder 200 em `/readyz`.
- `soak_sample.sh` — amostra memória do processo (tasklist) e conexões do Postgres durante
  um soak. Sem pprof/expvar expostos no binário, não mede goroutines (ver relatório).

## Regressão de performance no CI

Os benchmarks `testing.B` das funções quentes (RBAC, parse do fluxo) moram ao lado do
código (`internal/acesso/bench_test.go`, `internal/okf/fluxo_bench_test.go`), junto com um
`Test...NaoRegride` por benchmark: não é um benchmark, é uma asserção com limite bem
folgado (ordens de grandeza acima do medido em dev), feita para rodar em `go test` normal
no CI e só falhar numa regressão grosseira, nunca por ruído de máquina.
