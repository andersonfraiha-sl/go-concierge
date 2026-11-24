# Validação local da separação harness/core

Data: 24/11/2026. Branch `feat/separacao-harness-core`, base `c60bbea`, Go `go1.27.3 linux/amd64`. Resultados da árvore final antes do commit desta rodada. Os arquivos em `/tmp/concierge-harness-core/` preservam os logs locais; este documento registra o resultado durável.

Implementação concluída e pronta para o teste manual combinado. Os três apps permanecem separados e o Concierge mantém harness/core no mesmo processo, com transporte HTTP real para dados. Nenhuma chamada real LLM/B2C/Nexus/ATENDE foi necessária. PostgreSQL dos testes é descartável; a stack e os volumes preexistentes não foram reiniciados ou alterados.

## Verificações aprovadas

| Escopo | Evidência local |
|---|---|
| Fronteiras diretas e transitivas: harness sem core/Ent/SQL/pgx; apps visuais sem persistência/runtime | `layout-fronteiras.log` |
| Ent, sqlc e baseline v2 nos novos caminhos, sem mudança de schema | `geradores.log` |
| Compilação, builds dos executáveis, `go vet ./...`, golangci-lint v2.15.0 sem achados | `layout-compile-final.log`, `fechamento-build.log`, `fechamento-vet-final.log`, `fechamento-lint-final.log` |
| Dados HTTP por família: canal, conteúdo, sessões, memória, pedidos, auditoria, administração, catálogo, regras, entrega e retenção | Logs `dados-*.log`, `*-http-*.log`, `administracao-http-final.log`, `catalogo-saida-final.log`, `admin-consultas-motor.log` |
| Autorização administrativa por operação, revogação, integração com propósito separado e escopo empresa/canal/condomínio | `sessoes-admin-principal.log`, `contatos-admin-principal.log`, `auditoria-admin-principal.log`, `administracao-http-final.log`, `admin-consultas-motor.log` |
| Turno transacional, rollback integral, recusa recuperável, resposta perdida após commit e bloqueio de repetição | `dados-integracao-final.log`, `dados-recusa-recuperacao.log`, `sessao-resposta-perdida-real.log`, `pedidos-http-final.log` |
| Proteção de sessão inteira, cancelamento humano, concorrência e extrações pendentes | `sessao-protecao.log`, `sessao-protecao-race.log`, `sessao-cancelamento-final.log` |
| Regressões após ajustes de caminhos, injeção do relógio e modelos de teste | `fechamento-regressoes.log`, `smoke-processo-e-regressoes.log` (somente os pacotes com resultado `ok`; a falha inicial do processo foi corrigida abaixo) |
| CLI conectado pela API pública, permissões globais, credenciais restritas ao destino local e redirects recusados | `cli-http-principal-final.log`, `runtime-configuracao-final.log` |
| CLI offline, arquivos locais e LLM mock | `smoke-cli-offline-final.log` |
| Segredos nos candidatos Git e no histórico, com gitleaks v8.30.1 | `fechamento-segredos.log`, `fechamento-segredos-historico.log` |

Verificações finais executadas após a retomada:

```sh
TZ=UTC go test -p 1 -race ./internal/core/api/concierge -run '^TestProcessoConciergeUsaDadosHTTPNoBootEEncerra$' -count=1
TZ=UTC go test -p 1 -race ./internal/harness/atendimento -run '^TestAuditoriaNoLotePosResposta$' -count=1
gofmt -l internal/core/api/concierge/processo_test.go internal/harness/atendimento/auditoria_test.go
git diff --no-ext-diff --check
```

Todos encerraram com código zero. Logs dos dois testes: `smoke-processo-final.log` e `auditoria-lote-fechamento.log`. O processo de teste usa implantação global para exercer rotas globais; a fixture anterior fixava empresa e recebia 403 corretamente. A expectativa de consultas do teste de lote foi atualizada de três para nove, incluindo as validações de escopo; continuam verificadas gravação das partes, ausência de duplicação e recuperação da falha do lote.

## Suíte global e falhas preexistentes

Foi executado uma vez `TZ=UTC go test -p 1 -race ./...`, com log `fechamento-race.log`. Não houve relato de data race. Essa execução não passou: encontrou caminhos de fixtures após os movimentos, ajustes de compilação/relógio e as falhas anteriores abaixo. As regressões introduzidas foram corrigidas e validadas apenas nos pacotes/cenários afetados; a suíte global não foi repetida nem declarada verde.

Seis testes de topo continuam falhando também em `c60bbea`, comprovados em snapshot isolado da base com banco descartável, sem mudar a branch de trabalho:

| Teste | Diagnóstico comum à base e à rodada |
|---|---|
| `TestSessoesFicamNaImplantacao` | Fixture de sessão com colunas e JSON incompatíveis |
| `TestE2EGanchosRegrasDeExemplo` | Fixture de sessão fora do escopo autorizado |
| `TestAuditoriaDoBloqueioVaiAoBancoDepoisDoEnvio` | Expectativa em português difere dos enums persistidos, como `BEFORE_RESPONSE` e `BLOCK` |
| `TestAuditoriaDoEncerramentoGravaNaHoraUmaVez` | Espera evento em português; banco retorna `HANDOFF_OR_CLOSE` |
| `TestAuditoriaDoGanchoDeFerramentaVaiNoLote` | Espera evento em português; banco retorna `BEFORE_TOOL` |
| `TestAuditoriaDoPreencherPedidoVaiDepoisDoEnvio` | Espera evento em português; banco retorna `BEFORE_RESPONSE` |

Comando executado na base, com código final 1 e log `baseline-falhas.log`:

```sh
TZ=UTC go test -p 1 ./apps/concierge/cmd/concierge ./internal/atende -run '^(TestSessoesFicamNaImplantacao|TestE2EGanchosRegrasDeExemplo|TestAuditoriaDoBloqueioVaiAoBancoDepoisDoEnvio|TestAuditoriaDoEncerramentoGravaNaHoraUmaVez|TestAuditoriaNoLotePosResposta|TestAuditoriaDoGanchoDeFerramentaVaiNoLote|TestAuditoriaDoPreencherPedidoVaiDepoisDoEnvio)$' -count=1
```

O teste de lote passa na base e na árvore final com suas respectivas contagens; ele não integra as seis falhas anteriores. As assertivas legadas acima e o comportamento de produção correspondente foram preservados. A CI continuará acusando essas falhas até uma correção própria.

## Limites e próximo passo

- Teste manual com serviços externos ainda não executado nesta rodada. A imagem ativa pertence à rodada anterior.
- Não houve mudança de prompt, schema ou migrations históricas. Não é necessário reset de banco para testar o refactor.
- A fronteira roda no mesmo processo; locks e caches continuam locais. Não há suporte novo a múltiplas réplicas.
- Resultado de escrita incerto conserva o estado pendente e bloqueia repetição/fallback; não há nova garantia durável de idempotência após reinício.
- CLI autenticado remoto continua fora do contrato local. Nenhuma credencial da API privada é compartilhada com navegador ou CLI.
- O sandbox ainda falha com `bwrap: loopback: Failed RTM_NEWADDR: Operation not permitted`. Depois dos bloqueios nas duas sessões Terra 6.1 high, o principal concluiu com comandos locais elevados já autorizados. Erros exatos e momentos permanecem em `.temp/ERROS-FERRAMENTAS-HARNESS-CORE.md`.
