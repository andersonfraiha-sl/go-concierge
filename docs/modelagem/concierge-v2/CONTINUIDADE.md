# Continuidade do Concierge v2

Branch: `feat/refactor-identidade-canais-db`
Base do refactor: `8ad19a2` (baseline v2). Implementação e validação de fechamento concluídas; consulte `git log` para os commits desta etapa. Sem push e sem substituir bancos ou volumes preexistentes.

## Estado

O backend usa a persistência v2, com empresa Nexus obrigatória, canal persistido e conteúdo global → empresa → condomínio. Canais e ativações são configurados por SQL nesta etapa; não foram criadas APIs/telas de gestão. Referências: `internal/banco/v2/RUNTIME.md` e `docs/modelagem/concierge-v2/PERSISTENCIA-PESSOAL.md`.

`ValidarRegistroSessao` exige igualdade entre empresa, telefone e condomínio do JSON e das colunas. Studio e Copiloto usam `Caso.ID`; caso desconhecido mantém sua chave original. A documentação principal foi atualizada; o golden mudou apenas o exemplo de condomínio de `cond-a` para `101`. Código que perdeu seus chamadores foi removido após comparação do lint com a base.

Preservar sempre: empresa é o recorte mínimo; canal estrito Alpha exclui Beta e NULL; snapshots parciais/vazios nunca autorizam apagar outro alvo; identificadores Nexus, Gruvi e account continuam distintos. Não remodelar prompts nem iniciar separação de frontends nesta etapa.

Validação de fechamento em 18/11/2026:

- Uma execução de `TZ=UTC go test -p 1 -race ./...`; quatro expectativas antigas falharam e passaram em retestes focados com `-race` após correção. Sem repetir a suíte completa.
- Baseline `-check`, `go vet ./...`, golangci-lint v2.15.0 e formatação passaram. Ajustes finais de canais/lote tiveram testes focados com `-race`; CLI teve build e smoke.
- Studio: Vitest completo com dois workers passou (70 arquivos, 1.160 testes); Biome dos arquivos alterados passou. Typecheck/build aprovados anteriormente foram reutilizados.
- Gitleaks v8.30.1 passou no histórico e nos arquivos candidatos ao commit. O wrapper passou em sintaxe e smoke sintético: saída limitada a 80 linhas e código de falha preservado.
- Relatório detalhado de comandos, resultados e logs: `/tmp/concierge-validacao-fechamento.md`.

Evidências anteriores preservadas:

- `/tmp/concierge-sessao-v2-tests.log`: pacote `internal/atende` passou.
- `/tmp/concierge-studio-canonicos.log`: 3 arquivos e 49 testes passaram, incluindo `src/copiloto/aplicar.test.ts`.
- `/tmp/concierge-golden-v2.log`: `TestContratosDoStudio -args -atualizar-contrato` passou.
- `/tmp/concierge-studio-tests-final.log` e `/tmp/concierge-http-root-final.log` são anteriores às correções correspondentes e contêm falhas antigas; não representam o estado atual.

## Testes econômicos

Na raiz, `make` e `make test-focado` executam por padrão `go test ./internal/atende -run '^TestV2'`. Ajuste pacote e regex:

`make test-focado PKG=./internal/atende TEST='^TestV2SessaoAutorizaEscopoEValidaDados$'`

Para uma integração isolada, selecione explicitamente pacote e teste:

`make test-integracao INTEGRACAO_PKG=./internal/atende INTEGRACAO_TEST='^TestV2SessaoAutorizaEscopoEValidaDados$'`

A suíte completa é opt-in por `make test-completo`. Os comandos Go executam com `TZ=UTC`, gravam o log completo em `/tmp` (ou `LOG_DIR=...`) e preservam o código de saída. Em sucesso exibem apenas o resumo; em falha exibem até 80 linhas de diagnóstico e informam o caminho do log. Pacotes contendo `...` são recusados nos modos focado e integração. Nenhum alvo adiciona `-race`. Para a regressão de orientação do Studio: `cd web/studio && pnpm exec vitest run --maxWorkers=2 src/copiloto/aplicar.test.ts`.

## Próxima rodada

- Retomar pelos commits e por este handoff; não repetir discovery nem checks aprovados sem mudança relevante.
- O uso manual do runtime exige banco v2 dedicado e canal/ativações por SQL conforme `RUNTIME.md`. O banco preexistente não foi substituído nem populado por esta rodada.
- Definir com o usuário o próximo marco. APIs administrativas, integração visual do plugin e separação de frontends continuam etapas futuras.
