# Bateria de 500 pedidos — concierge (modelo real)

Gerada por `bateria/gerar.py`, executada por `bateria/rodar.py` contra o servidor local (webhook do ATENDE, identidade real do Gruvi, recursos pelo mock do concierge). Cada caso é um protocolo novo; cenários de vários turnos avaliam o último turno.

## Resumo

| Métrica | Valor |
|---|---|
| Casos | 500 |
| Sem nenhum problema | 451 (90.2%) |
| Rota errada | 46 (9.2%) |
| tool-error | 3 (0.6%) |
| turno-erro | 2 (0.4%) |
| resposta-vazia | 3 (0.6%) |
| falta | 2 (0.4%) |
| inventou-fora-de-escopo | 1 (0.2%) |
| Latência (último turno) | p50 6.2s · p90 9.9s · p95 13.6s · máx 34.7s · média 6.8s |
| Chamadas ao modelo por resposta | média 3.6 · máx 6 |
| Tokens por resposta | média 6697 · p95 10387 · máx 14186 |

## Por tipo

| tipo | Casos | OK | Rota errada | Erro na resposta | Latência p50 | p95 |
|---|---|---|---|---|---|---|
| acao | 133 | 97.7% | 3 | 0 | 6.7s | 11.9s |
| acao-fluxo | 9 | 77.8% | 1 | 0 | 4.9s | 18.9s |
| complexo | 38 | 68.4% | 12 | 0 | 12.8s | 28.9s |
| complexo-fluxo | 1 | 100.0% | 0 | 0 | 5.8s | 5.8s |
| consulta | 60 | 88.3% | 7 | 0 | 6.6s | 11.9s |
| consulta-fluxo | 1 | 100.0% | 0 | 0 | 5.8s | 5.8s |
| conteudo | 128 | 96.1% | 4 | 0 | 6.6s | 9.1s |
| conteudo-fluxo | 1 | 100.0% | 0 | 0 | 6.7s | 6.7s |
| fora | 39 | 87.2% | 4 | 0 | 3.7s | 7.9s |
| social | 52 | 98.1% | 1 | 0 | 2.9s | 4.2s |
| social-fluxo | 2 | 100.0% | 0 | 0 | 5.4s | 3.3s |
| transbordo | 35 | 60.0% | 14 | 0 | 3.7s | 4.2s |
| transbordo-fluxo | 1 | 100.0% | 0 | 0 | — | — |

## Por nível

| nível | Casos | OK | Rota errada | Erro na resposta | Latência p50 | p95 |
|---|---|---|---|---|---|---|
| 1 | 150 | 91.3% | 13 | 0 | 6.6s | 9.1s |
| 2 | 132 | 93.9% | 8 | 0 | 6.2s | 11.1s |
| 3 | 117 | 93.2% | 6 | 0 | 5.8s | 11.5s |
| 4 | 82 | 80.5% | 16 | 0 | 6.6s | 25.8s |
| 5 | 19 | 78.9% | 3 | 0 | 6.1s | 7.8s |

## Por intenção

| intenção | Casos | OK | Rota errada | Erro na resposta | Latência p50 | p95 |
|---|---|---|---|---|---|---|
| acao-fluxo | 9 | 77.8% | 1 | 0 | 4.9s | 18.9s |
| agendar-mudanca | 25 | 100.0% | 0 | 0 | 6.2s | 11.9s |
| armadilha | 15 | 80.0% | 3 | 0 | 6.2s | 7.8s |
| atualizacao-cadastral | 21 | 95.2% | 1 | 0 | 7.9s | 11.2s |
| codigo-retirada | 16 | 56.2% | 7 | 0 | 6.6s | 8.6s |
| codigo-retirada+liberacao-visitante | 3 | 0.0% | 3 | 0 | 18.2s | 18.2s |
| complexo-fluxo | 1 | 100.0% | 0 | 0 | 5.8s | 5.8s |
| comunicar-obra | 21 | 90.5% | 2 | 0 | 6.9s | 11.5s |
| consulta-fluxo | 1 | 100.0% | 0 | 0 | 5.8s | 5.8s |
| conteudo-fluxo | 1 | 100.0% | 0 | 0 | 6.7s | 6.7s |
| encomendas | 19 | 100.0% | 0 | 0 | 9.1s | 12.4s |
| encomendas+liberacao-visitante | 3 | 100.0% | 0 | 0 | 13.9s | 13.9s |
| faq | 76 | 94.7% | 4 | 0 | 6.6s | 9.1s |
| faq+encomendas | 2 | 0.0% | 2 | 0 | 31.8s | 28.9s |
| faq+reserva-area-comum | 3 | 66.7% | 1 | 0 | 11.1s | 11.1s |
| fora | 39 | 87.2% | 4 | 0 | 3.7s | 7.9s |
| liberacao-visitante | 30 | 100.0% | 0 | 0 | 5.6s | 6.7s |
| listar-unidades | 25 | 100.0% | 0 | 0 | 5.4s | 6.7s |
| listar-unidades+atualizacao-cadastral | 3 | 100.0% | 0 | 0 | 15.2s | 15.2s |
| regimento | 52 | 98.1% | 0 | 0 | 7.0s | 9.1s |
| regimento+comunicar-obra | 3 | 100.0% | 0 | 0 | 12.8s | 12.8s |
| regimento+reserva-area-comum | 3 | 100.0% | 0 | 0 | 19.7s | 19.7s |
| reserva-area-comum | 36 | 100.0% | 0 | 0 | 7.4s | 13.6s |
| reserva-area-comum+agendar-mudanca | 3 | 0.0% | 3 | 0 | 19.7s | 19.7s |
| social | 52 | 98.1% | 1 | 0 | 2.9s | 4.2s |
| social-fluxo | 2 | 100.0% | 0 | 0 | 5.4s | 3.3s |
| transbordo | 35 | 60.0% | 14 | 0 | 3.7s | 4.2s |
| transbordo-fluxo | 1 | 100.0% | 0 | 0 | — | — |

## Rotas erradas: esperada × obtida

| Esperada | Obtida | Vezes |
|---|---|---|
| transbordo | self | 12 |
| codigo-retirada | self | 5 |
| faq | transbordo | 4 |
| self|transbordo | faq | 4 |
| comunicar-obra | liberacao-visitante | 2 |
| codigo-retirada |  | 2 |
| reserva-area-comum|agendar-mudanca | self | 2 |
| codigo-retirada|liberacao-visitante | self | 2 |
| faq|encomendas | transbordo | 2 |
| transbordo | faq | 2 |
| atualizacao-cadastral | transbordo | 1 |
| faq|reserva-area-comum | transbordo | 1 |
| reserva-area-comum|agendar-mudanca | transbordo | 1 |
| codigo-retirada|liberacao-visitante | transbordo | 1 |
| reserva-area-comum|self | transbordo | 1 |
| faq|regimento|reserva-area-comum | transbordo | 1 |
| self | liberacao-visitante | 1 |
| codigo-retirada | transbordo | 1 |
| liberacao-visitante | self | 1 |

## Latência: os 10 mais lentos

| Caso | s | Rota | Chamadas | Pergunta |
|---|---|---|---|---|
| complexo-165 | 34.69 | transbordo | 4 | qual o endereço do condomínio? e também chegou algum pacote pra mim? |
| complexo-169 | 31.71 | self | 2 | preciso do código do pacote e também vai chegar um amigo meu hoje, pod |
| complexo-166 | 28.89 | transbordo | 4 | como cadastro minha digital na portaria? e também tenho alguma encomen |
| complexo-157 | 28.31 | comunicar-obra | 4 | o que o regimento diz sobre obras? e também vou trocar o piso, preciso |
| c-sem-perguntar-unidade | 25.77 | reserva-area-comum | 5 | quero reservar a quadra domingo às 10h |
| reserva-area-comum-421 | 22.58 | reserva-area-comum | 5 | Antes de tudo, parabéns pelo atendimento. Agora, a churrasqueira está  |
| agendar-mudanca-448 | 21.96 | agendar-mudanca | 3 | agendar mudança d entrada pro dia 3 |
| social-436 | 21.06 | liberacao-visitante | 3 | olá, aqui é o morador do 101. oi (pra hoje) |
| complexo-172 | 20.21 | reserva-area-comum | 4 | o que o regimento diz sobre obras? e também me reserva a churrasqueira |
| complexo-163 | 19.77 | transbordo | 3 | quero reservar a churrasqueira no sábado e também preciso marcar a dat |

## Exemplos de rota errada (46)

- **faq-018** · nível 1 · esperava `faq` → `transbordo` · 7.89s
  - pergunta: como cadastro minha digital na portaria?
  - resposta: Olá! Sou seu concierge virtual.  Entendi que você precisa cadastrar sua digital na portaria. Um atendente humano vai assumir este protocolo para te ajudar com isso.  Qualquer coisa, é só chamar por aqui. — Administração 
  - problemas: rota:transbordo≠faq
- **atualizacao-cadastral-062** · nível 1 · esperava `atualizacao-cadastral` → `transbordo` · 8.66s
  - pergunta: quero incluir minha esposa no cadastro
  - resposta: Olá! Eu sou o seu concierge.  Para incluir sua esposa no cadastro, um atendente da administração vai assumir este protocolo para te ajudar.  Qualquer coisa, é só chamar por aqui. — Administração do condomínio
  - problemas: rota:transbordo≠atualizacao-cadastral
- **comunicar-obra-079** · nível 1 · esperava `comunicar-obra` → `liberacao-visitante` · 10.63s
  - pergunta: o pintor vem semana que vem, como libero?
  - resposta: Olá! Sou o concierge do seu condomínio.  Para liberar a entrada do pintor, por favor, me informe:  *   **Nome completo do pintor:** *   **Telefone do pintor (com DDD):** *   **Motivo da visita:** *   **Data da visita:**
  - problemas: rota:liberacao-visitante≠comunicar-obra
- **codigo-retirada-093** · nível 1 · esperava `codigo-retirada` → `` · 2.9s
  - pergunta: me passa o código de retirada
  - resposta: 
  - problemas: rota:≠codigo-retirada, resposta-vazia, turno-erro
- **codigo-retirada-094** · nível 1 · esperava `codigo-retirada` → `self` · 6.59s
  - pergunta: preciso do código do pacote
  - resposta: Vanderlei, não consigo te ajudar com o código do pacote por aqui. Vou transferir seu atendimento para um de nossos atendentes. Eles vão assumir a partir de agora.
  - problemas: rota:self!=codigo-retirada, tool-error
- **codigo-retirada-095** · nível 1 · esperava `codigo-retirada` → `self` · 6.6s
  - pergunta: código pra pegar a encomenda na portaria
  - resposta: Olá, Vanderlei! Para assuntos de encomendas e códigos de retirada, por favor, entre em contato com a administração.  Um atendente humano vai assumir o protocolo para te ajudar com isso. 
  - problemas: rota:self!=codigo-retirada
- **fora-135** · nível 1 · esperava `self|transbordo` → `faq` · 6.59s
  - pergunta: qual o cnpj da superlógica?
  - resposta: Olá! Sou o concierge do condomínio.  Não encontrei o CNPJ da Superlógica. Para essa informação, sugiro que você entre em contato com a administração. Qualquer coisa, é só chamar por aqui. — Administração do condomínio
  - problemas: rota:faq!=self|transbordo
- **transbordo-142** · nível 1 · esperava `transbordo` → `self` · 2.9s
  - pergunta: não quero falar com robô
  - resposta: Entendido, Vanderlei. Vou transferir você para um atendente humano.  Encaminhamento: transferido ao atendente
  - problemas: rota:self!=transbordo
- **transbordo-143** · nível 1 · esperava `transbordo` → `self` · 2.92s
  - pergunta: cadê o síndico? quero falar com ele agora
  - resposta: Não consigo chamar o síndico diretamente, Vanderlei. Posso transferir seu pedido para um atendente humano. Eles poderão verificar como te ajudar.
  - problemas: rota:self!=transbordo
- **transbordo-146** · nível 1 · esperava `transbordo` → `self` · 2.88s
  - pergunta: tem alguém aí de verdade?
  - resposta: Olá, Vanderlei! Eu sou o concierge virtual do seu condomínio e posso te ajudar com dúvidas do regimento, do condomínio e a liberar visitantes.  Um atendente humano vai dar continuidade ao seu pedido. Qualquer coisa, é só
  - problemas: rota:self!=transbordo
- **transbordo-147** · nível 1 · esperava `transbordo` → `self` · 2.87s
  - pergunta: me liga, por favor, é urgente
  - resposta: Não consigo fazer ligações, Vanderlei. Vou transferir você para um de nossos atendentes.  Encaminhamento: transferido ao atendente
  - problemas: rota:self!=transbordo
- **transbordo-148** · nível 1 · esperava `transbordo` → `self` · 3.31s
  - pergunta: preciso falar com a administradora, é sobre uma cobrança indevida
  - resposta: Vanderlei, um atendente vai assumir seu protocolo para verificar a cobrança. 
  - problemas: rota:self!=transbordo
- … e mais 34.

## Exemplos de erro na resposta ou nas tools (5)

- **codigo-retirada-093** · nível 1 · esperava `codigo-retirada` → `` · 2.9s
  - pergunta: me passa o código de retirada
  - resposta: 
  - problemas: rota:≠codigo-retirada, resposta-vazia, turno-erro
- **codigo-retirada-094** · nível 1 · esperava `codigo-retirada` → `self` · 6.59s
  - pergunta: preciso do código do pacote
  - resposta: Vanderlei, não consigo te ajudar com o código do pacote por aqui. Vou transferir seu atendimento para um de nossos atendentes. Eles vão assumir a partir de agora.
  - problemas: rota:self!=codigo-retirada, tool-error
- **codigo-retirada-207** · nível 3 · esperava `codigo-retirada` → `self` · 4.95s
  - pergunta: me manda o código do armárrio
  - resposta: Não consigo te ajudar com o código do armário, Vanderlei. Posso ajudar com dúvidas do regimento, do condomínio ou a liberar visitantes. Se precisar de ajuda com o código do armário, por favor, entre em contato com a admi
  - problemas: rota:self!=codigo-retirada, tool-error
- **codigo-retirada-344** · nível 2 · esperava `codigo-retirada` → `` · 3.24s
  - pergunta: qual a senha de retirada do meu pacote??
  - resposta: 
  - problemas: rota:!=codigo-retirada, resposta-vazia, turno-erro
- **codigo-retirada-446** · nível 4 · esperava `codigo-retirada` → `self` · 4.14s
  - pergunta: Olha, eu moro aqui há 3 anos e nunca precisei disso, mas PRECISO DO CÓDIGO DO PACOTE
  - resposta: Vanderlei, para te ajudar com o código do pacote, preciso transferir seu atendimento para um de nossos atendentes. Eles conseguirão verificar essa informação para você.  Qualquer coisa, é só chamar por aqui. — Administra
  - problemas: rota:self!=codigo-retirada, tool-error

## Vazamento de termos internos (0)

Nenhum.

## Conteúdo esperado ausente ou indevido (3)

- **fora-459** · nível 3 · esperava `self|transbordo` → `self` · 3.32s
  - pergunta: então, quanto esta o dolar hoje?
  - resposta: Olá, Vanderlei! Eu sou o Concierge do condomínio e posso te ajudar com dúvidas do regimento, do condomínio ou a liberar visitantes.  Para informações sobre a cotação do dólar, sugiro que você consulte outras fontes de no
  - problemas: inventou-fora-de-escopo
- **c-obra-confirma** · nível 5 · esperava `comunicar-obra` → `comunicar-obra` · 4.53s
  - pergunta: sim
  - resposta: Certo, a obra será ruidosa.  Em qual(is) cômodo(s) será a pintura?
  - problemas: falta:MOCK-
- **c-visita-confirma** · nível 4 · esperava `liberacao-visitante` → `self` · 18.95s
  - pergunta: sim
  - resposta: Para liberar o João, preciso saber qual o motivo da visita dele.
  - problemas: rota:self!=liberacao-visitante, falta:MOCK-

## Timeouts e falhas de transporte (0)

Nenhum.

## Cenários de vários turnos (13)

- **c-reserva-confirma** · nível 4 · esperava `reserva-area-comum` → `reserva-area-comum` · 4.95s
  - pergunta: sim
  - resposta: Reserva da Churrasqueira confirmada para o dia 24/10/2026. O local é Jataí - Unidade Operacao Predial.  Seu protocolo é MOCK-935312. Qualquer coisa, é só chamar por aqui. — Administração do condomínio
  - problemas: 
- **c-reserva-nega** · nível 4 · esperava `reserva-area-comum|self` → `self` · 2.84s
  - pergunta: não, deixa pra lá
  - resposta: Tudo bem, Vanderlei! A reserva foi cancelada.  Qualquer coisa, é só chamar por aqui. — Administração do condomínio
  - problemas: 
- **c-obra-confirma** · nível 5 · esperava `comunicar-obra` → `comunicar-obra` · 4.53s
  - pergunta: sim
  - resposta: Certo, a obra será ruidosa.  Em qual(is) cômodo(s) será a pintura?
  - problemas: falta:MOCK-
- **c-mudanca-confirma** · nível 4 · esperava `agendar-mudanca` → `agendar-mudanca` · 4.93s
  - pergunta: sim
  - resposta: Sua mudança para Jataí - Unidade Operacao Predial em 25/10/2026, de manhã, foi agendada com sucesso! O protocolo é MOCK-706382. O elevador de serviço estará reservado para você. Qualquer coisa, é só chamar por aqui. — Ad
  - problemas: 
- **c-visita-confirma** · nível 4 · esperava `liberacao-visitante` → `self` · 18.95s
  - pergunta: sim
  - resposta: Para liberar o João, preciso saber qual o motivo da visita dele.
  - problemas: rota:self!=liberacao-visitante, falta:MOCK-
- **c-cadastro-confirma** · nível 5 · esperava `atualizacao-cadastral` → `atualizacao-cadastral` · 4.93s
  - pergunta: sim
  - resposta: Cadastro atualizado com sucesso. O número do protocolo é MOCK-177626. Qualquer coisa, é só chamar por aqui. — Administração do condomínio
  - problemas: 
- **c-encomenda-codigo** · nível 3 · esperava `codigo-retirada|encomendas` → `codigo-retirada` · 5.77s
  - pergunta: me passa o código da primeira
  - resposta: O código para retirar o Pacote Amazon (17/10) é *4831*. Por favor, apresente-o na portaria.  Qualquer coisa, é só chamar por aqui. — Administração do condomínio
  - problemas: 
- **c-transbordo-retem** · nível 4 · esperava `transbordo` → `transbordo` · 0.0s
  - pergunta: alguém aí?
  - resposta: 
  - problemas: 
- **c-saudacao-depois-pedido** · nível 2 · esperava `encomendas` → `encomendas` · 7.42s
  - pergunta: chegou pacote pra mim?
  - resposta: Sim, há encomendas para retirada: • Pacote Amazon (17/10) • Envelope Correios (16/10) Se precisar do código de retirada, é só pedir.
  - problemas: 
- **c-reserva-muda-data** · nível 5 · esperava `reserva-area-comum` → `reserva-area-comum` · 4.5s
  - pergunta: na verdade domingo, pode ser?
  - resposta: Confirma a reserva de Churrasqueira em 25 de outubro de 2026 para Jataí - Unidade Operacao Predial? (sim/não)
  - problemas: 
- **c-faq-depois-regimento** · nível 3 · esperava `regimento|faq` → `regimento` · 6.66s
  - pergunta: e pode levar cachorro lá?
  - resposta: Não é permitida a permanência de animais na piscina, conforme o Art. 20 do regimento interno.  Qualquer coisa, é só chamar por aqui. — Administração do condomínio
  - problemas: 
- **c-despedida** · nível 2 · esperava `self|encomendas` → `self` · 3.3s
  - pergunta: valeu, era só isso!
  - resposta: De nada, Vanderlei!  Qualquer coisa, é só chamar por aqui. — Administração do condomínio
  - problemas: 
- **c-dois-pedidos-sequencia** · nível 5 · esperava `liberacao-visitante` → `liberacao-visitante` · 5.76s
  - pergunta: beleza, e libera minha mãe amanhã às 15h
  - resposta: Para liberar a entrada da sua mãe amanhã às 15h, preciso que você me informe o nome completo dela e o telefone.  Amanhã será segunda-feira, 19 de outubro de 2026. Confirma o horário das 15h?
  - problemas: 

