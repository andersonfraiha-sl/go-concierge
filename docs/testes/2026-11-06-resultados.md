# Resultado dos testes: o que deu certo, o que deu errado e o que descobrimos

Fechado em 06/11/2026. Resume todas as rodadas de teste do go-concierge até aqui. O detalhe de cada uma está em [`2026-11-05-bateria.md`](2026-11-05-bateria.md), a matriz dos 261 tipos de teste em [`2026-11-05-matriz.md`](2026-11-05-matriz.md) e o que falta testar em [`2026-11-06-anderson.md`](2026-11-06-anderson.md).

## tltr

- **Deu certo:** back, API, segurança e Studio passam no CI. A bateria de conversa com modelo real chegou a 95,6% na rodada 2. As 5 skills novas passam ponta a ponta com a Gruvi simulada.
- **Deu errado:** 10 problemas, todos corrigidos com teste. Duas falhas do Studio seguem abertas.
- **Descobrimos:** o harness caía quando o modelo mandava argumento de tool no formato errado. A matriz de 05/11 dizia mais do que o código tem: não há retry nem circuit breaker, e a carga não tem número registrado.
- **Não testado:** conversa com LLM real desde 18/10, skills novas contra a Gruvi real, carga, deploy.

## Rodadas

| Rodada | Quando | Ambiente | Resultado |
|---|---|---|---|
| Conversa, 500 casos, rodada 1 | 18/10 | Gemini `2.5-flash-lite`, Gruvi real para identidade | 451 sem problema (90,2%), 46 rotas erradas, 5 erros de tool ou turno |
| Conversa, 500 casos, rodada 2 | 18/10 | igual, depois das correções | 478 sem problema (95,6%), 18 rotas erradas, 0 erros |
| 261 tipos de teste | 05/11, conferida em 06/11 | local, Postgres real, Playwright | 41 testados, 113 parciais, 28 não testados, 49 dependem de gente ou produção, 30 não se aplicam |
| Skills de engenharia reversa | 06/11 | testes Go, Gruvi simulada | 5 skills passando ponta a ponta; CI verde nas PRs #9, #10, #11 |
| Busca de erros com IA local | 06/11 | Ollama `llama3.1:8b`, RTX 4070 Laptop, 527 casos | 2 crashes achados e corrigidos; depois disso 0 turnos caindo |

## O que deu certo

- **Banco:** isolamento de transação (lost update, write skew, deadlock), constraints e migrations up/down contra Postgres real.
- **Segurança da API:**
  - matriz de permissão por papel em todas as rotas;
  - uma administradora não lê nem escreve recurso de outra (IDOR);
  - validação de entrada, injeção, headers e corpo gigante;
  - CPF, telefone, e-mail e token fora do log;
  - gitleaks, `gosec`, trivy e `govulncheck` limpos.
- **Webhook do Atende:** evento duplicado, fora de ordem e reenviado não duplica turno.
- **Studio:**
  - 1157 testes unitários;
  - E2E nas jornadas principais;
  - axe em todas as telas, navegação só por teclado;
  - regressão visual e responsividade de 375 a 1920.
- **Conversa sem LLM:**
  - prompt injection não vira instrução de sistema;
  - ferramentas ignoram `account` injetado e não vazam entre condomínios;
  - reformular a pergunta não muda a rota.
- **Conversa com LLM (18/10):** transbordo foi de 60% para 97% e dois pedidos na mesma mensagem de 68% para 82%. Social e consulta ficaram em 100%.
- **Skills novas** (`boleto`, `pagamento`, `debito`, `documentos`, `chamado`):
  - com a Gruvi simulada, cada uma traz o dado certo e sai como morador do condomínio do canal;
  - o `chamado` só abre depois do "sim" e não abre duas vezes;
  - a trava B2C reescreve `persona` e `companyId` vindos do modelo.

## O que deu errado (e foi corrigido)

| # | Problema | Como apareceu | Correção |
|---|---|---|---|
| 1 | Gravação de arquivo do okf não atômica: queda no meio deixava `agent.yaml` truncado | revisão de crash consistency | grava em temporário e renomeia |
| 2 | Caminho com byte nulo passava na validação | fuzzing | rejeita byte nulo |
| 3 | `down` da migration `acesso_ajustes` falhava | teste de migration reversível | ordem do DROP corrigida |
| 4 | Webhook reprocessava evento reentregue e duplicava o turno | teste de idempotência | guarda antes de qualquer efeito |
| 5 | 15 rotas aceitavam corpo de qualquer tamanho | teste de payload gigante | teto único, 413 acima do limite |
| 6 | 5 cores de texto abaixo do contraste AA | axe | tons ajustados |
| 7 | `ler_dado` com `pagina` em texto (`"1"`) derrubava o turno | bateria com IA local | aceita número ou texto |
| 8 | `chamar_recurso` com `params` em texto JSON derrubava o turno (18 casos) | bateria com IA local | aceita objeto ou texto |
| 9 | `rodar.py` mandava para o webhook real, que exige HMAC | bateria com IA local | usa o webhook do emulador |
| 10 | Testes instáveis: ganchos e catálogo medidos no relógio de parede, data dependente do fuso da máquina | rodadas repetidas e CI em UTC | relógio injetado, data no fuso de São Paulo |

Na conversa, entre as rodadas 1 e 2 de 18/10, também foram corrigidos:
- o workflow citava uma tool inexistente;
- o concierge dizia "vou transferir" sem transferir;
- o código de retirada roteava errado;
- dois pedidos na mesma mensagem não atendiam nenhum.

## O que segue errado

1. **Lista numerada vazia derruba a tela do Concierge no Studio.** O `@tiptap/markdown` lança ao ler item vazio. O teste `monkey.e2e.ts` fica vermelho até a correção.
2. **CLS do Concierge acima de 0,1.**
3. **18 rotas erradas na rodada 2**, metade em mensagens com dois pedidos. O que a FAQ não cobre vira transbordo.
4. **`c-visita-confirma` reprovado no validador da bateria**: o telefone fictício parece dado real para o detector.
5. **`bateria/gerar.py` não é reprodutível**: `hash()` muda a cada processo.

## O que descobrimos

- **O formato dos argumentos de tool é um ponto frágil.** Quando o modelo erra o tipo de um argumento, o parse falha antes da tool e o turno inteiro cai. O morador recebe a mensagem de fallback. Modelo pequeno erra mais, mas modelo grande também erra. Os dois casos que apareceram estão corrigidos; outras tools com argumento tipado podem ter o mesmo problema.
- **O `rodar.py` não enxerga esse crash.** O caso aparece como `resposta-de-erro`, não como `turno-erro`. O sinal confiável é `turno do concierge falhou` no log do servidor.
- **IA local serve para achar erro, não para medir qualidade.** O `llama3.1:8b` acertou 29% das rotas e inventa dado. Custo zero de token, mas 14 s por caso (2h10 para a bateria) numa RTX 4070 Laptop de 8 GB.
- **A resiliência é menor do que a matriz de 05/11 dizia.** Não há retry com backoff nem circuit breaker; existem timeout e fallback.
- **Carga não tem número.** Os scripts k6 existem, mas nenhum resultado está registrado.
- **Muita coisa só rodou à mão, uma vez.** Playwright, `govulncheck`, `pnpm audit`, trivy e SBOM estão fora do CI.
- **Latência cresce linear com o número de chamadas ao modelo** (~1,5 s cada). O extrator de memória está no caminho crítico.
- **Sem Gruvi não há morador identificado**, então os casos de recurso privado (unidades, encomendas, código de retirada) só são testáveis com identidade real.
