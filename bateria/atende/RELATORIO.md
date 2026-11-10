# Benchmark do concierge com conversas reais do ATENDE

100 pedidos reais de moradores (ATENDE, outubro/2026), reescritos sem dados pessoais: **50 que os agentes do concierge deveriam resolver** e **50 que deveriam ir para o atendente** (transbordo). Casos em `casos.json`; cada um aponta a conversa de origem e o que aconteceu nela no ATENDE.

Critério: no grupo *resolve*, acerta quem responde pela rota esperada sem transbordar; no grupo *transbordo*, acerta quem passa a conversa ao atendente (ou vai a uma rota aceita no caso, como o débito que resume e oferece o atendente). "Acerto limpo" exige também nenhum problema técnico (timeout, resposta vazia, erro de tool, vazamento de termo interno).

**Ambiente:** bancada local, sem Gruvi (nenhum morador identificado). Os agentes que consultam dado privado (boleto, débito, pagamento, documentos...) chegam ao caso certo, mas a consulta é barrada pelo gancho `ferramenta_permitida` e o concierge responde que não conseguiu. Esses casos contam como **rota certa** e aparecem na coluna *dado bloqueado*; o acerto limpo os desconta. A rota vem dos passos do turno (motor de pedidos ou roteador), e cada caso usa um telefone próprio, para a memória do morador não passar de um caso a outro.

## Resumo

| Rodada | Casos | Resolve: rota certa | ↳ dado bloqueado | Transbordo: certo | Total (rota) | Acerto limpo | Latência p50 / p95 |
|---|---|---|---|---|---|---|---|
| Local (qwen2.5 7B) | 100 | 36/50 (72%) | 33 | 30/50 (60%) | 66/100 (66%) | 45/100 (45%) | 9.65 s / 59.9 s |
| Luna (gpt-6-luna) | 100 | 40/50 (80%) | 36 | 35/50 (70%) | 75/100 (75%) | 39/100 (39%) | 21.58 s / 51.17 s |
| Luna, política nova | 100 | 41/50 (82%) | 37 | 47/50 (94%) | 88/100 (88%) | 54/100 (54%) | 15.78 s / 23.65 s |

## Concierge × ATENDE

No export, *bot resolveu* quer dizer só que a conversa fechou sem humano: inclui morador que desistiu, menu repetido e atendimento encerrado por inatividade.

### Local (qwen2.5 7B)

| Grupo | Desfecho no ATENDE | Casos | Concierge acertou (rota) |
|---|---|---|---|
| resolve | bot-resolveu | 26 | 21 (81%) |
| resolve | humano | 24 | 15 (62%) |
| transbordo | bot-resolveu | 23 | 16 (70%) |
| transbordo | humano | 27 | 14 (52%) |

- **Atendimento poupado:** 15 pedidos que no ATENDE precisaram de humano e o concierge resolveu sozinho.
- **Morador não abandonado:** 16 pedidos que no ATENDE fecharam sem humano (e sem solução) e o concierge passou ao atendente.

### Luna (gpt-6-luna)

| Grupo | Desfecho no ATENDE | Casos | Concierge acertou (rota) |
|---|---|---|---|
| resolve | bot-resolveu | 26 | 23 (88%) |
| resolve | humano | 24 | 17 (71%) |
| transbordo | bot-resolveu | 23 | 16 (70%) |
| transbordo | humano | 27 | 19 (70%) |

- **Atendimento poupado:** 17 pedidos que no ATENDE precisaram de humano e o concierge resolveu sozinho.
- **Morador não abandonado:** 16 pedidos que no ATENDE fecharam sem humano (e sem solução) e o concierge passou ao atendente.

### Luna, política nova

| Grupo | Desfecho no ATENDE | Casos | Concierge acertou (rota) |
|---|---|---|---|
| resolve | bot-resolveu | 26 | 24 (92%) |
| resolve | humano | 24 | 17 (71%) |
| transbordo | bot-resolveu | 23 | 23 (100%) |
| transbordo | humano | 27 | 24 (89%) |

- **Atendimento poupado:** 17 pedidos que no ATENDE precisaram de humano e o concierge resolveu sozinho.
- **Morador não abandonado:** 23 pedidos que no ATENDE fecharam sem humano (e sem solução) e o concierge passou ao atendente.

## Por intenção

| Grupo | Intenção | Casos | Local (qwen2.5 7B) | Luna (gpt-6-luna) | Luna, política nova |
|---|---|---|---|---|---|
| resolve | agendar-mudanca | 4 | 3/4 | 3/4 | 2/4 |
| resolve | atualizacao-cadastral | 3 | 3/3 | 0/3 | 0/3 |
| resolve | boleto | 12 | 11/12 | 12/12 | 12/12 |
| resolve | chamado | 4 | 4/4 | 4/4 | 4/4 |
| resolve | comunicar-obra | 1 | 0/1 | 0/1 | 0/1 |
| resolve | debito | 4 | 3/4 | 3/4 | 3/4 |
| resolve | documentos | 3 | 3/3 | 2/3 | 3/3 |
| resolve | faq | 3 | 0/3 | 2/3 | 2/3 |
| resolve | liberacao-visitante | 5 | 5/5 | 5/5 | 5/5 |
| resolve | pagamento | 4 | 1/4 | 4/4 | 4/4 |
| resolve | regimento | 3 | 0/3 | 3/3 | 3/3 |
| resolve | reserva-area-comum | 4 | 3/4 | 2/4 | 3/4 |
| transbordo | acesso-app | 4 | 4/4 | 4/4 | 4/4 |
| transbordo | acordo | 4 | 2/4 | 4/4 | 4/4 |
| transbordo | aluguel | 5 | 3/5 | 3/5 | 5/5 |
| transbordo | cancelar-reserva | 3 | 1/3 | 3/3 | 1/3 |
| transbordo | certidao | 5 | 1/5 | 3/5 | 5/5 |
| transbordo | contestacao | 8 | 4/8 | 4/8 | 8/8 |
| transbordo | dp-fiscal | 3 | 2/3 | 2/3 | 3/3 |
| transbordo | falar-com-alguem | 5 | 5/5 | 5/5 | 5/5 |
| transbordo | gestao | 3 | 2/3 | 2/3 | 3/3 |
| transbordo | notificacao | 2 | 2/2 | 1/2 | 2/2 |
| transbordo | outras-cobrancas | 2 | 0/2 | 1/2 | 2/2 |
| transbordo | titularidade | 6 | 4/6 | 3/6 | 5/6 |

## Para onde cada pedido foi (Luna, política nova)

| Intenção | Rota obtida | Casos |
|---|---|---|
| acesso-app | transbordo | 4 |
| acordo | debito | 1 |
| acordo | pagamento | 1 |
| acordo | transbordo | 2 |
| agendar-mudanca | agendar-mudanca | 2 |
| agendar-mudanca | transbordo | 2 |
| aluguel | transbordo | 5 |
| atualizacao-cadastral | transbordo | 3 |
| boleto | boleto | 12 |
| cancelar-reserva | reserva-area-comum | 2 |
| cancelar-reserva | transbordo | 1 |
| certidao | transbordo | 5 |
| chamado | chamado | 4 |
| comunicar-obra | transbordo | 1 |
| contestacao | transbordo | 8 |
| debito | boleto | 1 |
| debito | debito | 2 |
| debito | transbordo | 1 |
| documentos | documentos | 3 |
| dp-fiscal | transbordo | 3 |
| falar-com-alguem | transbordo | 5 |
| faq | boleto | 1 |
| faq | faq | 2 |
| gestao | transbordo | 3 |
| liberacao-visitante | liberacao-visitante | 5 |
| notificacao | transbordo | 2 |
| outras-cobrancas | transbordo | 2 |
| pagamento | pagamento | 4 |
| regimento | documentos | 2 |
| regimento | regimento | 1 |
| reserva-area-comum | reserva-area-comum | 3 |
| reserva-area-comum | transbordo | 1 |
| titularidade | atualizacao-cadastral | 1 |
| titularidade | transbordo | 5 |

## Erros (Luna, política nova)

Fora daqui os casos de rota certa com dado bloqueado (falta de identidade da bancada, não erro do agente).

| Caso | Pedido | Esperado | Obtido | Motivo | Resposta (início) |
|---|---|---|---|---|---|
| r20-debito | cara, eu acabei pulando um mês do condomínio e não percebi. quanto está em aberto? | debito\|boleto | transbordo (transbordo) | transbordou (devia resolver) | Olá, sou o concierge do condomínio.  Não consegui concluir o atendimento sobre débito e acordo. Não consigo consultar as cobranças da unidade por este telefone. |
| r29-faq | o boleto do condomínio vence que dia? | faq | boleto | rota boleto (esperada faq) | Olá, sou o concierge do condomínio. Não consegui concluir a emissão da segunda via do boleto. A administração poderá ajudar com essa solicitação. |
| r37-reserva-area-comum | eu tentei fazer a reserva do salão e da churrasqueira no app mas dá erro e pede para entrar em contato com a administradora | reserva-area-comum | transbordo (transbordo) | transbordou (devia resolver) | Olá, sou o concierge do condomínio. Como seu contato não está identificado, não consigo acessar suas unidades nem concluir as reservas do salão e da churrasquei |
| r41-agendar-mudanca | como faço para agendar uma mudança? | agendar-mudanca | transbordo (transbordo) | transbordou (devia resolver) | Olá! Sou o concierge do condomínio. Para agendar a mudança, é necessário acessar as unidades vinculadas ao seu cadastro, mas seu telefone não está identificado |
| r42-agendar-mudanca | amanhã tem como fazer a mudança? | agendar-mudanca | transbordo (transbordo) | transbordou (devia resolver) | Olá! Sou o concierge do condomínio. Não consigo confirmar a disponibilidade para amanhã, 11/11. Registrei uma solicitação para a administração verificar isso co |
| r43-atualizacao-cadastral | olá, estou solicitando a atualização dos meus dados de cadastro, mudei de e-mail | atualizacao-cadastral | transbordo (transbordo) | transbordou (devia resolver) | Não consigo acessar seu cadastro porque este contato não está identificado pelo telefone. Registrei a solicitação para atendimento humano; a transferência ainda |
| r44-atualizacao-cadastral | gostaria de alterar o e-mail de recebimento do boleto do condomínio | atualizacao-cadastral | transbordo (transbordo) | transbordou (devia resolver) | Para alterar o e-mail de recebimento do boleto, preciso acessar os dados da unidade, mas este contato não está identificado pelo telefone. Registrei a solicitaç |
| r45-atualizacao-cadastral | boa noite! troquei de carro, a placa nova é QWE1R23 | atualizacao-cadastral | transbordo (transbordo) | transbordou (devia resolver) | Não consigo acessar nem alterar o cadastro porque este contato não está identificado. Registrei a solicitação para atendimento, que precisará validar sua identi |
| r50-comunicar-obra | referente à minha unidade, quero fazer uma reforma. o que preciso apresentar? | comunicar-obra\|regimento | transbordo (transbordo) | transbordou (devia resolver) | Olá, sou o concierge da administração. Não tenho a lista de documentos exigidos para reformas. Registrei seu pedido para a administração confirmar os requisitos |
| t06-cancelar-reserva | vocês podem cancelar essa reserva ou me orientar como proceder para não ser cobrada a utilização? | transbordo | reserva-area-comum | não transbordou (rota reserva-area-comum) | Olá, sou o concierge do condomínio. Este canal não consegue cancelar a reserva nem confirmar como evitar a cobrança. A Administração pode verificar as regras e |
| t08-cancelar-reserva | reservei a churrasqueira para o aniversário da minha filha, mas ela vai ter compromisso. tentei cancelar pelo aplicativo e não consegui | transbordo | reserva-area-comum | não transbordou (rota reserva-area-comum) | Olá, sou o concierge do condomínio. Não consigo cancelar reservas por aqui. Para receber orientação sobre o cancelamento, fale com a administração. |
| t23-titularidade | o inquilino da unidade 143 não é mais o mesmo, ele saiu | transbordo | atualizacao-cadastral | não transbordou (rota atualizacao-cadastral) | Olá, sou o concierge do condomínio. Não consigo acessar ou atualizar os dados da unidade sem identificar você pelo telefone. Entre em contato pelo número cadast |

## Problemas técnicos por rodada

| Rodada | timeout | resposta-vazia | tool-error | turno-erro | resposta-de-erro | vazamento |
|---|---|---|---|---|---|---|
| Local (qwen2.5 7B) | 0 | 0 | 6 | 0 | 20 | 8 |
| Luna (gpt-6-luna) | 0 | 0 | 0 | 0 | 43 | 0 |
| Luna, política nova | 0 | 0 | 0 | 0 | 36 | 0 |
