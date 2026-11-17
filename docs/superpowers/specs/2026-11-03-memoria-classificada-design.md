# Concierge: memória classificada

Data: 2026-11-03 · Revisão: 2026-11-10 · Autor: Felipe
Status: decidido em 10/11/2026

## 1. Por que

Hoje a memória do concierge é uma lista de fatos com chave livre, inventada pelo modelo a cada troca ("pets", "unidades", "familia"). Ela não sabe o que a pessoa costuma pedir, não tem prazo e cresce sem regra. A ideia é que o concierge **lembre o que importa, ofereça o próximo passo e esqueça o resto**.

Exemplo do comportamento esperado:

> **Morador:** oi
> **Concierge:** (memória: pediu a segunda via do boleto da unidade 12 nos últimos três meses; nenhum pedido em aberto)
> **Concierge:** Oi, Marcos! Quer a segunda via do boleto da unidade 12, como das outras vezes? Se for outra coisa, me fala.
> **Morador:** sim
> **Concierge:** (abre o pedido de segunda via para a unidade 12, pelo fluxo de sempre)

## 2. Objetivos, não objetivos e anti objetivos

**Objetivos**
- Não pedir de novo o que a pessoa já disse sobre como quer ser atendida.
- Oferecer o próximo passo quando o hábito é claro e a mensagem não diz o que a pessoa quer.
- Esquecer sozinho: o que não é usado perde peso e some.

**Não objetivos**
- Substituir o cadastro. Identidade e vínculo (quem é, de qual condomínio e unidade, com qual papel) vêm do Gruvi/ERP a cada turno. A memória guarda **uso**, não **verdade cadastral**.
- Memória de conversa social ou de personalidade.

**Anti objetivos (falha mesmo funcionando)**
- Oferecer ou executar algo numa unidade a que a pessoa não tem vínculo no turno atual.
- Guardar CPF, telefone ou qualquer texto livre do morador.
- Executar uma ação porque a memória achou que era isso.
- Deixar o turno mais lento.

## 3. Tipos de memória

Cada coisa lembrada tem **um tipo**. O tipo define de onde vem, onde mora e quanto vive.

| Tipo | O que é | Exemplo | Fonte | Vida |
|---|---|---|---|---|
| `padrao_de_uso` | contador com decaimento por caso de uso, condomínio e unidade | segunda via do boleto, Residencial Ipê, unidade 12 | pedido concluído no turno | decai com meia-vida de 45 dias; sai quando o valor cai abaixo de 0,1 |
| `preferencia` | como a pessoa quer ser atendida, de uma lista fechada | prefere resposta curta | o que o morador disse, extraído em segundo plano | 180 dias sem reafirmar |
| `pendencia` | pedido em aberto na carteira de pedidos | agendamento de mudança aguardando confirmação | carteira de pedidos (sem guarda própria) | enquanto o pedido estiver aberto |
| contexto de sessão | o que a conversa atual já tem: histórico, plano, oferta feita | a oferta do turno anterior | sessão | a sessão |

Fora da memória, vindos da identidade do turno: **identidade** e **vínculo**. A memória guarda só referência (id do condomínio e da unidade) para contar padrão. Nada que um modelo extrai vira identidade, vínculo ou autorização.

### Preferências da lista

| Chave | Valores |
|---|---|
| `nome_preferido` | um nome curto: só letras, até duas palavras, até 30 caracteres |
| `tamanho_resposta` | `curta`, `detalhada` |
| `tratamento` | `informal` (você), `formal` (senhor, senhora) |
| `formato_boleto` | `linha_digitavel`, `pix`, `link` |

Chave fora da lista, ou valor fora dos permitidos, é descartado na extração. A lista cresce por decisão de produto, nunca pelo modelo.

## 4. Como classificar

- **Caminho quente, por regra.** O turno já tem sinal estruturado: a skill que atendeu, o caso de uso do pedido, os recursos executados e o desfecho. Um pedido que termina **concluído** no turno é uma ocorrência de `padrao_de_uso` com o caso, o condomínio da conversa e a unidade do pedido. Nenhuma chamada de modelo a mais antes da resposta.
- **Segundo plano, com o modelo barato que já existe.** Depois da resposta, o extrator lê a troca e devolve só preferências, num esquema fechado (chaves e valores da seção 3). O que vier fora é descartado.
- **Jev depois.** A classificação de ocorrência fica atrás de uma interface. A primeira versão é a regra; o Jev pode entrar no lugar sem mudar leitura, ranking nem oferta.

Resumo: **a regra classifica, o código conta, o modelo barato só extrai preferência, o LLM escreve.**

## 5. Ranking

Cada padrão guarda `(valor, atualizado_em)`. A cada ocorrência:

```
valor = valor · 0,5^(Δdias / meia_vida) + 1
```

- Meia-vida de **45 dias**. Sem log de eventos: atualizar é O(1).
- Para ler, o valor é decaído até agora com a mesma fórmula.
- Duas ocorrências do mesmo padrão em menos de 12 horas contam uma.
- **Oferecer** quando o primeiro candidato tem valor **≥ 2,0** e é **≥ 2×** o segundo.
- **Recusa** da oferta divide o valor por 2 e suprime a oferta daquele padrão por **14 dias**.
- **Limpeza diária** apaga padrão com valor decaído abaixo de **0,1** e preferência com mais de **180 dias** sem reafirmar.

Todos os números são configuráveis e são ponto de partida, a calibrar com a taxa de aceite real.

## 6. Onde aparece

| Lugar | O que mostra |
|---|---|
| Prompt do turno | um bloco curto, marcado como dado e não instrução: as preferências, até 3 padrões da unidade e do condomínio da conversa, e os pedidos em aberto |
| Oferta | quando a mensagem é saudação ou vaga, não há pedido em aberto e há um padrão forte: o bloco pede ao concierge que pergunte e espere |
| Aba Memória do Console | cada item com o tipo; no padrão, o valor e a data; esquecer uma preferência ou tudo |
| Rotas de leitura (`/api/memoria`, `/api/memorias` e as mesmas da integração) | os mesmos itens com tipo, valor e data |

Sem tela nova no Studio.

## 7. Antecipar é perguntar

- Com padrão forte, o concierge **oferece** ("quer a segunda via do boleto da unidade 12?") e espera.
- **Sim** curto logo depois da oferta abre o pedido daquele caso, com a unidade oferecida, e segue o fluxo normal do caso. Escrita continua pedindo a confirmação do próprio fluxo.
- **Não** curto é recusa (seção 5). Qualquer outra resposta só deixa a oferta expirar, sem penalidade.
- A oferta só cita unidade que está no vínculo do turno atual. Sem vínculo confirmado, só padrão sem unidade pode ser oferecido.
- Executar sem perguntar fica fora da primeira versão.

### Exemplos

**A. Hábito claro, saudação.** O exemplo da seção 1.

**B. Dois hábitos parecidos.** Segunda via (valor 2,4) e reserva do salão (valor 1,8). 2,4 não é o dobro de 1,8: não oferece, responde a saudação normal.

**C. Pedido em aberto.** "oi" com um agendamento aguardando confirmação: a pendência vem primeiro, não há oferta.

**D. Recusou.** "não, é outra coisa": a segunda via cai para metade e não é oferecida por 14 dias.

**E. Unidade que saiu do vínculo.** O padrão da unidade 12 existe, mas o Gruvi hoje só devolve a unidade 15: o padrão não aparece no prompt nem vira oferta.

## 8. Esquecer e limpeza

- **Esquecer tudo** (rota que já existe, inclusive a da integração) apaga preferências, padrões e o cache do contato numa chamada. A conversa em andamento não grava de volta o que foi esquecido.
- **Esquecer uma preferência** pela chave continua.
- Os fatos de chave livre de hoje fora da lista **somem da leitura** na hora e a limpeza os apaga. Nada é migrado para tipo novo por adivinhação.

## 9. LGPD e segurança

- Chave de tudo: administradora + telefone, como hoje. A leitura e o cache são por essa chave.
- Nenhum texto livre novo é guardado: padrão é id e número; preferência é valor de lista (o nome preferido é a única exceção, curto e validado).
- CPF e telefone continuam mascarados pelos ganchos antes de qualquer extração.
- **Memória envenenada:** o modelo só preenche campos da lista; ocorrência só nasce de pedido concluído pelo motor; nada da memória autoriza ação nem vira vínculo; o bloco do prompt entra como dado.
- Logs levam o contato mascarado, o tipo e o caso, nunca o conteúdo da conversa.

## 10. Latência (regra dura)

- Padrões e preferências entram no mesmo cache por contato e no mesmo lote a frio de hoje.
- A atualização do contador vai no lote depois do envio.
- Contato quente: **0 idas ao banco antes da resposta**. Contato frio: **1**, como hoje.
- Nenhuma chamada de modelo nova antes da resposta.

## 11. O que o mercado faz

Pesquisa completa em `docs/superpowers/pesquisas/2026-11-10-memoria-de-agentes.md`.

- Vertex Memory Bank, AgentCore, LangMem e Letta escrevem memória em segundo plano; escrever no caminho quente custa latência. Aqui só o contador vai no lote pós envio, sem modelo.
- LangMem separa perfil (estado atual) de coleção (fatos soltos). Identidade e vínculo são perfil e vêm do ERP.
- Decagon trata o CRM como verdade e a memória como complemento. É a regra 2 deste desenho.
- MINJA e o OWASP (ASI06) mostram que filtro de conteúdo não segura memória envenenada; a defesa que sobra é arquitetural: esquema fechado e nada do modelo vira vínculo.
- Generative Agents usa decaimento exponencial para recência. O contador com meia-vida é a mesma família, em O(1).
- Benchmarks (LoCoMo, LongMemEval) medem pergunta e resposta em conversa longa, não antecipação. A métrica nossa é taxa de aceite e de recusa da oferta.

## 12. Decisões de 10/11/2026

1. **Sem Jev na primeira versão; regra no caminho quente, modelo barato em segundo plano só para preferência.** Porque o turno já tem sinal estruturado e qualquer chamada antes da resposta fere a regra de latência.
2. **Identidade e vínculo nunca vêm da memória.** Porque memória envenenada não pode virar acesso a dado de outra unidade.
3. **Antecipar é perguntar, não executar.** Porque oferecer errado custa uma frase; executar errado custa uma escrita ou um vazamento.
4. **Tipos: `padrao_de_uso`, `preferencia`, `pendencia` derivada da carteira, contexto de sessão. Cortados: episódio em texto livre, grafo, vetor, agente que edita a própria memória.** Porque é o mínimo que a oferta precisa, sem texto livre.
5. **Fatos soltos aceitam só chaves da lista; os antigos fora dela somem da leitura e são apagados.** Porque adivinhar o tipo de um fato antigo é o mesmo erro da chave livre.
6. **Contador com decaimento exponencial, O(1), sem log de eventos, com os números da seção 5.** Porque responde "frequente e recente" num número só, sem crescer.
7. **Padrões e preferências no cache e no lote de hoje; contador no lote pós envio.** Porque mantém 0 idas no contato quente.
8. **Chave por administradora + telefone; esquecer apaga tudo; nenhum texto livre novo.** Porque LGPD pede apagar por titular e não guardar o que não precisa.
9. **Bloco curto no prompt, oferta na saudação, itens com tipo no Console e nas rotas; sem tela no Studio.** Porque o valor está no atendimento, não numa tela nova.

## 13. Decisões do construtor

Pontos em que o código pediu uma escolha. Todas pela opção mais simples que respeita latência e identidade.

1. **O condomínio vem do canal, não da memória.** O turno resolve o condomínio pelo mapeamento antes de tudo. A pergunta "qual condomínio" do rascunho sai; o padrão conta dentro do condomínio da conversa e a oferta só olha esse condomínio.
2. **Ocorrência = pedido concluído pelo motor de pedidos.** Os casos de uso do catálogo têm contrato de pedido; skill sem contrato (dúvidas, regimento) e conversa solta não contam. É o único sinal com desfecho estruturado.
3. **Unidade do padrão = o parâmetro `unidade` do pedido.** Pedido sem esse parâmetro conta sem unidade. Na leitura, padrão de unidade fora do vínculo do turno é ignorado.
4. **12 horas entre ocorrências do mesmo padrão.** Uma conversa sozinha não fabrica hábito (eventos independentes, recomendação da pesquisa contra envenenamento).
5. **Recusa divide por 2** (a decisão deixava zerar ou dividir). Dividir mantém a linha com a supressão até a limpeza.
6. **Saudação ou vaga por regra sobre o texto**, de uma lista fechada de frases curtas. Oferta só com o motor de pedidos ligado e sem pedido em aberto.
7. **Sim e não curtos logo depois da oferta são lidos por regra.** O sim vira o pedido em código, sem depender do extrator inferir pelo histórico.
8. **`nome_preferido` não cabe numa lista.** Fica como a única exceção: nome curto validado.
9. **Revalidar preferência = o morador reafirmar.** Reafirmar renova a data. Sem pergunta ativa de revalidação na primeira versão.
10. **Pendência não ganha item nas rotas de memória.** Ela aparece no prompt e já está na rota e na aba de pedidos.
11. **Sem competência do boleto.** A oferta cita o caso e a unidade; o mês fica para quando o ERP der o sinal.
12. **A limpeza não mexe no cache.** A leitura já esconde preferência vencida e padrão fraco; o cache de padrão abaixo de 0,1 nunca vira oferta.
13. **A interface de classificador cobre só a ocorrência.** Sim e não da oferta são regra pura; trocar pelo Jev depois não muda essa parte.

## 14. Metas

- Contato quente: 0 idas ao banco antes da resposta; frio: 1. Guarda automática no teste de idas do turno.
- Nenhuma chamada de modelo nova antes da resposta.
- Esquecer tudo deixa a leitura seguinte vazia, inclusive com uma conversa em andamento.
- Nenhuma oferta cita unidade fora do vínculo do turno.
- Cada oferta e cada resposta a ela ficam em log de negócio (tipo, caso, aceite ou recusa), para medir taxa de aceite e de recusa.

## 15. Fora da primeira versão

- Executar sem perguntar.
- Episódio em texto livre, resumo de conversa, grafo, vetor e busca semântica.
- Agente que edita a própria memória.
- Resolução de conflito por modelo.
- Jev (entra depois pela interface do classificador).
- Competência e ciclo mensal do boleto.
- Esquecer um padrão isolado pelo Console.
- Tela no Studio.

## 16. Estado em 10/11/2026

**Pronto na `main`:** memória por contato no Postgres com cache e lote a frio; extrator com modelo barato em segundo plano; carteira de pedidos; CPF e telefone mascarados pelos ganchos; esquecer pelas rotas e pela integração.

**Falta:** tudo das seções 3 a 10. Plano em `docs/superpowers/plans/2026-11-10-memoria-classificada.md`.
