# Análise da bateria de 500 pedidos (2026-10-18)

Modelo real (Gemini, `gemini-2.5-flash-lite` no planejador), identidade real do Gruvi para o
telefone de teste, recursos pelo mock do concierge, 4 conversas em paralelo. Números
completos em `RELATORIO.md`; casos em `casos.json`; respostas em `resultados.jsonl`.

## O quadro

| | |
|---|---|
| Casos | 500 (150 simples, 132 com variação de estilo, 117 com erros de digitação/contexto, 82 complexos, 19 armadilhas; 15 cenários de vários turnos) |
| Sem problema algum | 451 (90,2%) |
| Rota errada | 46 (9,2%), concentradas em três lugares (abaixo) |
| Falhas duras (resposta vazia por erro do harness) | 2 (0,4%) |
| Vazamento de termo interno | 0 depois de descontar o `MOCK-` do protocolo do mock |
| Latência | p50 6,2 s · p90 9,9 s · p95 13,6 s · máx 34,7 s |

Por tipo: social 98%, conteúdo (FAQ e regimento) 96%, ações simples 98%, consultas 88%,
fora de escopo 74% (a maioria falso positivo do meu próprio critério, corrigido no
relatório), pedidos de humano 60%, mensagens com dois pedidos 68%.

## Os achados, do mais grave ao menos

### 1. Instrução antiga no workflow do concierge derruba a resposta (2 casos, bug)

Dois pedidos de código de retirada terminaram sem resposta, com erro do harness
`tool use_case not found`. O `workflows/principal.md` do concierge ainda diz "devolva a
ação `use_case` com o id do caso: o harness executa os passos do YAML", texto herdado do
sl-concierge. O modelo tentou chamar uma tool com esse nome. Correção: reescrever o passo
2 do workflow para "encaminhe pela rota" e tirar qualquer menção a `use_case`.

### 2. O concierge diz que transfere, mas não transfere (12 de 35 pedidos de humano)

"não quero falar com robô", "me liga, é urgente", "quero registrar uma denúncia": o
texto responde "Vou transferir você para um atendente. Encaminhamento: transferido ao
atendente", mas a tool `transferir_atendente` não foi chamada, então o protocolo segue
com o robô. A causa é o contrato de saída (`rules/output.md`), que tem uma seção
"Encaminhamento" onde escrever "transferido ao atendente" parece bastar. Correções, em
camadas: (a) no contrato, dizer que a transferência só acontece pela tool; (b) guarda
determinística no harness: resposta que anuncia transferência sem a tool ter sido chamada
dispara o transbordo mesmo assim. Sete respostas da bateria cairiam nessa guarda.

### 3. Código de retirada é a rota mais frágil (7 de 16 errados)

Além dos dois erros do item 1, em três casos o orquestrador respondeu ele mesmo e chamou
`chamar_recurso` com o nome da rota (`codigo-retirada`) como se fosse um recurso, deu
erro e desistiu ("vou transferir"). O roteador não levou ao subagente, apesar da linha
de rota existir. Correções: o mesmo item 1 (o passo 2 induz o orquestrador a "executar"
o caso), e a descrição do caso `codigo-retirada` ganhar os termos que apareceram na
bateria ("senha", "armário", "código do pacote").

### 4. Dois pedidos na mesma mensagem (12 de 38 errados, e as respostas mais lentas)

"quero reservar a churrasqueira no sábado e também preciso marcar a mudança": em cinco
casos o concierge transferiu para humano só porque havia dois pedidos; em outros cinco
perguntou "qual dos dois primeiro?" sem atender nenhum (contei como rota errada, mas é
uma UX defensável); as latências foram de 13 a 35 s, com até seis chamadas ao modelo e
flips de rota em cadeia. Correção: política explícita "com dois pedidos, atenda o
primeiro pela rota e ofereça o segundo em seguida; nunca transfira por isso".

### 5. O que a FAQ não cobre vira transbordo (8 casos)

"como cadastro minha digital na portaria?", "quanto custa reservar o salão?", "já
reservei a churrasqueira?": sem resposta no conhecimento, o concierge transferiu. É o
comportamento desenhado ("humano quando não dá"), então não é bug; é cobertura. Vale
ampliar a FAQ do condomínio de exemplo com esses assuntos e, no caso de "já reservei?",
um recurso de consulta de reservas.

### 6. Cenários de vários turnos

Reserva, mudança e cadastro com "sim" terminaram com protocolo. Dois ficaram pelo
caminho: a comunicação de obra, porque a primeira mensagem não tinha todos os dados e
o "sim" não era resposta a um resumo (o agente seguiu coletando, correto); e a
liberação de visitante, em que depois do "sim" o agente ainda perguntou o motivo, que é
opcional, e a conversa caiu para o orquestrador (19 s). A regra "pergunte o mínimo"
precisa cobrir parâmetro opcional: não pergunte o que o recurso não exige.

### 7. Latência

A latência é linear no número de chamadas ao modelo: 2 chamadas ≈ 3,3 s, 4 ≈ 6,6 s,
6 ≈ 9,5 s, cerca de 1,5 s por chamada. Uma resposta roteada custa quatro: roteador,
subagente, entrega do orquestrador e extrator de memória. O prompt é pesado: 6,7 mil
tokens em média por resposta, 10 mil nas reservas e no código de retirada (projeção
inteira mais índice de conhecimento mais contextos). Onde cortar, por ordem de ganho:
- extrator de memória fora do caminho crítico (responder antes de extrair);
- pular a passada de entrega quando o subagente já devolveu a resposta final sem
  assets ou persona a aplicar;
- roteador com prompt mínimo (só rotas e glossário) e modelo mais rápido;
- índice de conhecimento por `index.md` (progressive disclosure) em vez do índice
  completo, que já existe para pacotes OKF e pode valer para o catálogo do concierge.

### 8. Miudezas

- Fora de escopo: nenhuma invenção real; o concierge recusa e oferece o que faz. O CNPJ
  da Superlógica foi procurar na FAQ (rota `faq`), aceitável.
- Uma saudação com preâmbulo ("olá, aqui é o morador do 101. oi (pra hoje)") caiu em
  liberação de visitante.
- Uma resposta do regimento voltou vazia sem erro registrado ("visitante pode usar a
  academia?" com preâmbulo): o modelo leu o regimento e não redigiu. Vale um guarda
  no harness: resposta vazia vira "não encontrei" em vez de silêncio.

## O que fazer primeiro

1. Workflow do concierge sem `use_case` (item 1): elimina os dois crashes e boa parte
   do item 3.
2. Transferência só pela tool, com a guarda por texto (item 2).
3. Política para dois pedidos (item 4).
4. Extrator de memória assíncrono e entrega condicional (item 7).
5. Cobertura da FAQ e termos do código de retirada (itens 3 e 5).

Depois disso, rodar a bateria de novo (`python bateria/rodar.py` é retomável e grava em
`resultados.jsonl`; apague o arquivo para começar do zero) e comparar os números.

## Segunda rodada (depois das correções, 2026-10-18)

Mesmos 500 casos, mesmo modelo, depois do commit f3bdc10 (workflow sem `use_case`,
transbordo garantido pela guarda de texto, política para dois pedidos, termos do código
de retirada, extração de memória fora do caminho crítico, entrega pulada em pergunta
curta, resposta vazia vira aviso). Tabela completa em `COMPARACAO.md`; primeira rodada
preservada em `resultados-rodada1.jsonl` e `RELATORIO-rodada1.md`.

| Métrica | Rodada 1 | Rodada 2 |
|---|---|---|
| Sem problema | 451 (90,2%) | 478 (95,6%) |
| Rota errada | 46 (9,2%) | 18 (3,6%) |
| Falhas duras (vazia, erro do harness, erro de tool) | 8 | 0 |
| Latência p50 / p90 / p95 | 6,2 / 9,9 / 13,6 s | 5,4 / 8,7 / 10,3 s |
| Pedidos de humano transferidos de fato | 60% | 97% |
| Consultas (unidades, encomendas, código) | 88% | 100% |
| Dois pedidos na mesma mensagem | 68% | 82% (p50 de 12,8 s para 7,4 s) |

Melhoraram 37 casos, pioraram 10, 12 continuam com problema. Nenhuma resposta vazia,
nenhum crash do harness, nenhum `chamar_recurso` com nome de rota.

### O que ainda falha (22 casos) e por quê

- **Cobertura, não bug (12).** "Como cadastro minha digital na portaria?" (5 variações),
  "já reservei a churrasqueira?", "corrigir meu nome / incluir minha esposa no cadastro"
  (3), "quanto custa o salão?", "avisa a portaria que chega um pacote": o conhecimento
  não tem a resposta ou o recurso não tem o campo (o cadastro de exemplo só tem e-mail,
  placa e pets), e o concierge transfere ao humano, como foi desenhado. O que resolve é
  conteúdo: FAQ com esses assuntos e `cadastro.campos` com nome e moradores.
- **Critério meu, não do concierge (3).** CNPJ da Superlógica foi procurado na FAQ
  (rota `faq`) e respondido "não encontrei": certo. "Preciso falar com a administradora"
  recebeu o telefone da administradora em vez de transbordo: aceitável.
- **Dois pedidos (2).** Ainda transfere quando a segunda metade é uma ação ("qual o
  horário de silêncio? e também dá pra agendar o salão"). A política está no texto; o
  modelo barato nem sempre a segue. Um roteador que corta a mensagem no "e também" e
  trata só a primeira parte resolveria no determinístico.
- **Erros do modelo na cadeia longa (2).** Duas reservas terminaram em "não consegui
  processar" depois de 17 s e 34 s: a cadeia roteador → subagente → recursos → entrega
  passou do tempo do provedor numa chamada e o agente desistiu. Vale um retry curto por
  chamada e o teto de iterações mais baixo.
- **Parâmetro opcional perguntado (1).** A liberação de visitante ainda pergunta o motivo,
  que o recurso não exige. A regra "pergunte o mínimo" precisa de uma linha explícita:
  parâmetro opcional não se pergunta.
- **Coleta legítima (1).** A comunicação de obra continua coletando depois do "sim",
  porque a primeira mensagem não trazia tipo, prazo e prestadores.
- **Preâmbulo confunde a rota (1).** "oi, tudo bem? vou trocar o piso, preciso avisar?"
  foi ao regimento em vez da comunicação de obra; a resposta até orienta, mas não abre o
  caso.

### Latência

A queda de p50 veio da extração de memória fora do caminho crítico e da entrega pulada
nas perguntas curtas; o número de chamadas por resposta não mudou (3,6) porque a
extração ainda conta no uso, só não atrasa a resposta. Os 10 s de p95 são as cadeias
com flip de rota e entrega. Próximo corte com mais efeito: roteador com prompt mínimo e
índice de conhecimento por `index.md`.
