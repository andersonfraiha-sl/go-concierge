# Bateria de 500 pedidos — concierge (modelo real)

Gerada por `bateria/gerar.py`, executada por `bateria/rodar.py` contra o servidor local (webhook do ATENDE, identidade real do Gruvi, recursos pelo mock do concierge). Cada caso é um protocolo novo; cenários de vários turnos avaliam o último turno.

## Resumo

| Métrica | Valor |
|---|---|
| Casos | 500 |
| Sem nenhum problema | 478 (95.6%) |
| Rota errada | 18 (3.6%) |
| resposta-de-erro | 3 (0.6%) |
| falta | 2 (0.4%) |
| Latência (último turno) | p50 5.4s · p90 8.7s · p95 10.3s · máx 33.9s · média 5.6s |
| Chamadas ao modelo por resposta | média 3.6 · máx 6 |
| Tokens por resposta | média 6913 · p95 10691 · máx 16037 |

## Por tipo

| tipo | Casos | OK | Rota errada | Erro na resposta | Latência p50 | p95 |
|---|---|---|---|---|---|---|
| acao | 133 | 95.5% | 5 | 2 | 5.8s | 12.9s |
| acao-fluxo | 9 | 77.8% | 0 | 0 | 4.2s | 5.0s |
| complexo | 38 | 81.6% | 6 | 1 | 7.4s | 14.5s |
| complexo-fluxo | 1 | 100.0% | 0 | 0 | 9.5s | 9.5s |
| consulta | 60 | 100.0% | 0 | 0 | 5.8s | 8.7s |
| consulta-fluxo | 1 | 100.0% | 0 | 0 | 5.3s | 5.3s |
| conteudo | 128 | 96.9% | 4 | 0 | 6.2s | 9.0s |
| conteudo-fluxo | 1 | 100.0% | 0 | 0 | 5.3s | 5.3s |
| fora | 39 | 94.9% | 2 | 0 | 3.3s | 6.2s |
| social | 52 | 100.0% | 0 | 0 | 2.5s | 3.4s |
| social-fluxo | 2 | 100.0% | 0 | 0 | 3.3s | 1.7s |
| transbordo | 35 | 97.1% | 1 | 0 | 3.3s | 3.7s |
| transbordo-fluxo | 1 | 100.0% | 0 | 0 | — | — |

## Por nível

| nível | Casos | OK | Rota errada | Erro na resposta | Latência p50 | p95 |
|---|---|---|---|---|---|---|
| 1 | 150 | 99.3% | 1 | 0 | 5.4s | 8.7s |
| 2 | 132 | 94.7% | 6 | 2 | 5.4s | 10.9s |
| 3 | 117 | 96.6% | 4 | 0 | 5.0s | 9.0s |
| 4 | 82 | 92.7% | 5 | 0 | 5.8s | 13.3s |
| 5 | 19 | 78.9% | 2 | 1 | 5.8s | 9.1s |

## Por intenção

| intenção | Casos | OK | Rota errada | Erro na resposta | Latência p50 | p95 |
|---|---|---|---|---|---|---|
| acao-fluxo | 9 | 77.8% | 0 | 0 | 4.2s | 5.0s |
| agendar-mudanca | 25 | 100.0% | 0 | 0 | 5.9s | 9.6s |
| armadilha | 15 | 80.0% | 2 | 1 | 6.2s | 9.0s |
| atualizacao-cadastral | 21 | 85.7% | 3 | 0 | 6.6s | 10.9s |
| codigo-retirada | 16 | 100.0% | 0 | 0 | 6.2s | 7.5s |
| codigo-retirada+liberacao-visitante | 3 | 100.0% | 0 | 0 | 7.9s | 7.9s |
| complexo-fluxo | 1 | 100.0% | 0 | 0 | 9.5s | 9.5s |
| comunicar-obra | 21 | 95.2% | 1 | 0 | 5.8s | 7.5s |
| consulta-fluxo | 1 | 100.0% | 0 | 0 | 5.3s | 5.3s |
| conteudo-fluxo | 1 | 100.0% | 0 | 0 | 5.3s | 5.3s |
| encomendas | 19 | 100.0% | 0 | 0 | 7.1s | 10.0s |
| encomendas+liberacao-visitante | 3 | 100.0% | 0 | 0 | 10.3s | 10.3s |
| faq | 76 | 94.7% | 4 | 0 | 6.2s | 9.0s |
| faq+encomendas | 2 | 50.0% | 1 | 0 | 9.7s | 8.3s |
| faq+reserva-area-comum | 3 | 66.7% | 1 | 0 | 9.1s | 9.1s |
| fora | 39 | 94.9% | 2 | 0 | 3.3s | 6.2s |
| liberacao-visitante | 30 | 100.0% | 0 | 0 | 4.1s | 5.8s |
| listar-unidades | 25 | 100.0% | 0 | 0 | 5.0s | 6.3s |
| listar-unidades+atualizacao-cadastral | 3 | 100.0% | 0 | 0 | 6.2s | 6.2s |
| regimento | 52 | 100.0% | 0 | 0 | 6.2s | 7.5s |
| regimento+comunicar-obra | 3 | 100.0% | 0 | 0 | 8.3s | 8.3s |
| regimento+reserva-area-comum | 3 | 33.3% | 2 | 0 | 14.5s | 14.5s |
| reserva-area-comum | 36 | 94.4% | 1 | 2 | 7.5s | 15.7s |
| reserva-area-comum+agendar-mudanca | 3 | 100.0% | 0 | 0 | 7.0s | 7.0s |
| social | 52 | 100.0% | 0 | 0 | 2.5s | 3.4s |
| social-fluxo | 2 | 100.0% | 0 | 0 | 3.3s | 1.7s |
| transbordo | 35 | 97.1% | 1 | 0 | 3.3s | 3.7s |
| transbordo-fluxo | 1 | 100.0% | 0 | 0 | — | — |

## Rotas erradas: esperada × obtida

| Esperada | Obtida | Vezes |
|---|---|---|
| faq | transbordo | 4 |
| atualizacao-cadastral | transbordo | 3 |
| regimento|reserva-area-comum | transbordo | 2 |
| self|transbordo | faq | 2 |
| faq|reserva-area-comum | self | 1 |
| faq|encomendas | transbordo | 1 |
| reserva-area-comum|self | transbordo | 1 |
| encomendas|liberacao-visitante|self | transbordo | 1 |
| comunicar-obra | regimento | 1 |
| reserva-area-comum | transbordo | 1 |
| transbordo | faq | 1 |

## Latência: os 10 mais lentos

| Caso | s | Rota | Chamadas | Pergunta |
|---|---|---|---|---|
| reserva-area-comum-195 | 33.92 | transbordo | 5 | reserva a quadra pra domingo de manhã kkk |
| complexo-167 | 20.58 | codigo-retirada | 3 | preciso do código do pacote e também quero cadastrar uma visita pra se |
| agendar-mudanca-448 | 18.5 | agendar-mudanca | 4 | agendar mudança d entrada pro dia 3 |
| reserva-area-comum-388 | 17.07 | reserva-area-comum | 5 | olá, aqui é o morador do 101. queria bater um churras no próximo sábad |
| reserva-area-comum-213 | 15.73 | reserva-area-comum | 5 | Antes de tudo, parabéns pelo atendimento. Agora, RESERVA O SALÃO PRA S |
| reserva-area-comum-275 | 15.29 | reserva-area-comum | 5 | desculpa incomodar, por favor, reserva o salão pra sexta à noite |
| complexo-170 | 14.87 | transbordo | 5 | qual o horário de silêncio? e também dá pra agendar o salão de festas  |
| complexo-171 | 14.52 | regimento | 4 | pode bicicleta no elevador? e também preciso da quadra na quarta às 19 |
| reserva-area-comum-046 | 13.73 | reserva-area-comum | 4 | quero reservar a churrasqueira no sábado |
| faq-295 | 13.64 | faq | 4 | onde fica a administração? (pra hoje) |

## Exemplos de rota errada (18)

- **faq-018** · nível 1 · esperava `faq` → `transbordo` · 4.97s
  - pergunta: como cadastro minha digital na portaria?
  - resposta: Olá! Sou o concierge do seu condomínio.  Para te ajudar com o cadastro da digital na portaria, um atendente humano vai assumir este protocolo.  Qualquer coisa, é só chamar por aqui. — Administração do condomínio
  - problemas: rota:transbordo!=faq
- **complexo-155** · nível 4 · esperava `faq|reserva-area-comum` → `self` · 9.08s
  - pergunta: como cadastro minha digital na portaria? e também reserva a quadra pra domingo de manhã
  - resposta: Olá, Vanderlei!  Para cadastrar sua digital na portaria, você precisa comparecer pessoalmente na administração do condomínio em horário comercial.  Sobre a reserva da quadra, por favor, me diga diretamente "quero reserva
  - problemas: rota:self!=faq|reserva-area-comum
- **complexo-166** · nível 4 · esperava `faq|encomendas` → `transbordo` · 8.29s
  - pergunta: como cadastro minha digital na portaria? e também tenho alguma encomenda?
  - resposta: Olá! Sou seu concierge virtual.  Para cadastrar sua digital na portaria e verificar se há encomendas, vou transferir você para um atendente humano. Ele poderá te ajudar com essas questões.  Qualquer coisa, é só chamar po
  - problemas: rota:transbordo!=faq|encomendas
- **complexo-170** · nível 4 · esperava `regimento|reserva-area-comum` → `transbordo` · 14.87s
  - pergunta: qual o horário de silêncio? e também dá pra agendar o salão de festas dia 12/11?
  - resposta: Olá! Sou o concierge do condomínio.  Um atendente da administração vai assumir seu protocolo para te ajudar com suas solicitações. Qualquer coisa, é só chamar por aqui. — Administração do condomínio
  - problemas: rota:transbordo!=regimento|reserva-area-comum
- **complexo-172** · nível 4 · esperava `regimento|reserva-area-comum` → `transbordo` · 9.97s
  - pergunta: o que o regimento diz sobre obras? e também me reserva a churrasqueira pra amanhã
  - resposta: Olá! Sou o concierge do condomínio.  Sobre as obras, o regimento permite de segunda a sexta, das 8h às 17h, e aos sábados das 9h às 13h. Não são permitidas obras aos domingos e feriados. É importante comunicar a administ
  - problemas: rota:transbordo!=regimento|reserva-area-comum
- **armadilha-175** · nível 5 · esperava `reserva-area-comum|self` → `transbordo` · 9.07s
  - pergunta: já reservei a churrasqueira? não lembro
  - resposta: Olá! Sou o concierge do condomínio.  Para te ajudar a verificar suas reservas existentes, um atendente da administração vai assumir o atendimento por aqui.  Qualquer coisa, é só chamar por aqui. — Administração do condom
  - problemas: rota:transbordo!=reserva-area-comum|self
- **armadilha-177** · nível 5 · esperava `encomendas|liberacao-visitante|self` → `transbordo` · 7.4s
  - pergunta: depois de amanhã chega um pacote, avisa a portaria
  - resposta: Olá! Sou seu concierge virtual.  Recebemos sua solicitação para avisar a portaria sobre a chegada de um pacote. Um de nossos atendentes dará continuidade a este pedido, pois ele exige um acompanhamento mais específico.  
  - problemas: rota:transbordo!=encomendas|liberacao-visitante|self
- **comunicar-obra-192** · nível 2 · esperava `comunicar-obra` → `regimento` · 13.22s
  - pergunta: oi, tudo bem? vou trocar o piso, preciso avisar?
  - resposta: Olá! Sou o concierge do seu condomínio.  Sim, você precisa comunicar a administração sobre a troca do piso antes de iniciar a obra, informando os nomes dos prestadores de serviço.  Obras são permitidas de segunda a sexta
  - problemas: rota:regimento!=comunicar-obra
- **reserva-area-comum-195** · nível 2 · esperava `reserva-area-comum` → `transbordo` · 33.92s
  - pergunta: reserva a quadra pra domingo de manhã kkk
  - resposta: Olá! Sou o concierge virtual do condomínio.  Não consegui processar a sua solicitação de reserva da quadra para domingo de manhã exatamente como você pediu. As informações que encontrei não correspondem à sua solicitação
  - problemas: rota:transbordo!=reserva-area-comum, resposta-de-erro
- **atualizacao-cadastral-215** · nível 3 · esperava `atualizacao-cadastral` → `transbordo` · 9.14s
  - pergunta: preciso corrigir meu nome no cdastro
  - resposta: Olá! Eu sou o concierge virtual do seu condomínio.  Sobre a correção do seu nome, eu não consigo fazer essa alteração por aqui. Minha função é ajudar com a atualização de dados como e-mail, informações de veículo ou pets
  - problemas: rota:transbordo!=atualizacao-cadastral
- **atualizacao-cadastral-220** · nível 3 · esperava `atualizacao-cadastral` → `transbordo` · 7.86s
  - pergunta: preciso corrigir meu nome no cadastro (pra hoje)
  - resposta: Olá! Sou seu concierge virtual.  Para a correção do seu nome no cadastro, um atendente vai assumir este protocolo para te ajudar.  Qualquer coisa, é só chamar por aqui. — Administração do condomínio
  - problemas: rota:transbordo!=atualizacao-cadastral
- **atualizacao-cadastral-254** · nível 2 · esperava `atualizacao-cadastral` → `transbordo` · 9.03s
  - pergunta: quero incluir minha esposa no cadastro urgente
  - resposta: Olá! Sou o concierge do seu condomínio.  Para incluir sua esposa no cadastro, um de nossos atendentes dará continuidade ao seu pedido.  Qualquer coisa, é só chamar por aqui. — Administração do condomínio
  - problemas: rota:transbordo!=atualizacao-cadastral
- … e mais 6.

## Exemplos de erro na resposta ou nas tools (3)

- **armadilha-187** · nível 5 · esperava `listar-unidades|self` → `listar-unidades` · 6.57s
  - pergunta: meu apartamento é o 101 ou o 102? esqueci
  - resposta: Olá! Sou seu concierge virtual.  Pelo seu cadastro, identificamos a unidade "Jataí - Unidade Operacao Predial". Não conseguimos confirmar se é a 101 ou 102 por aqui.  Para essa informação específica, por favor, entre em 
  - problemas: resposta-de-erro
- **reserva-area-comum-195** · nível 2 · esperava `reserva-area-comum` → `transbordo` · 33.92s
  - pergunta: reserva a quadra pra domingo de manhã kkk
  - resposta: Olá! Sou o concierge virtual do condomínio.  Não consegui processar a sua solicitação de reserva da quadra para domingo de manhã exatamente como você pediu. As informações que encontrei não correspondem à sua solicitação
  - problemas: rota:transbordo!=reserva-area-comum, resposta-de-erro
- **reserva-area-comum-388** · nível 2 · esperava `reserva-area-comum` → `reserva-area-comum` · 17.07s
  - pergunta: olá, aqui é o morador do 101. queria bater um churras no próximo sábado
  - resposta: Olá! Sou o concierge do condomínio.  Morador do 101, não consegui processar sua solicitação de reserva de churrasqueira. Houve um problema com as informações da data ou da unidade.  Para que possamos te ajudar, por favor
  - problemas: resposta-de-erro

## Vazamento de termos internos (0)

Nenhum.

## Conteúdo esperado ausente ou indevido (2)

- **c-visita-confirma** · nível 4 · esperava `liberacao-visitante` → `liberacao-visitante` · 2.51s
  - pergunta: sim
  - resposta: Para liberar o João, preciso saber o motivo da visita. Qual seria?
  - problemas: falta:MOCK-
- **c-obra-confirma** · nível 5 · esperava `comunicar-obra` → `comunicar-obra` · 4.14s
  - pergunta: sim
  - resposta: Para registrar a comunicação da obra, ainda preciso que você me diga:  *   Qual será a descrição da pintura? (Ex: Pintura de paredes internas, pintura da fachada, etc.) *   Quais cômodos serão pintados? *   A obra será r
  - problemas: falta:MOCK-

## Timeouts e falhas de transporte (0)

Nenhum.

## Cenários de vários turnos (13)

- **c-reserva-confirma** · nível 4 · esperava `reserva-area-comum` → `reserva-area-comum` · 3.71s
  - pergunta: sim
  - resposta: Reserva confirmada: Churrasqueira em 24/10/2026 (Jataí - Unidade Operacao Predial). Protocolo: MOCK-248463. Qualquer coisa, é só chamar por aqui. — Administração do condomínio
  - problemas: 
- **c-reserva-nega** · nível 4 · esperava `reserva-area-comum|self` → `self` · 2.88s
  - pergunta: não, deixa pra lá
  - resposta: Tudo bem, Vanderlei! Se precisar de algo mais, estou à disposição para liberar visitantes, tirar dúvidas do regimento ou do condomínio.  Protocolo: 20261018-000001 Qualquer coisa, é só chamar por aqui. — Administração do
  - problemas: 
- **c-mudanca-confirma** · nível 4 · esperava `agendar-mudanca` → `agendar-mudanca` · 4.57s
  - pergunta: sim
  - resposta: Sua mudança para 25/10/2026, de manhã, foi agendada com sucesso! Protocolo: MOCK-806437. O elevador de serviço estará reservado para você. Qualquer coisa, é só chamar por aqui. — Administração do condomínio
  - problemas: 
- **c-visita-confirma** · nível 4 · esperava `liberacao-visitante` → `liberacao-visitante` · 2.51s
  - pergunta: sim
  - resposta: Para liberar o João, preciso saber o motivo da visita. Qual seria?
  - problemas: falta:MOCK-
- **c-cadastro-confirma** · nível 5 · esperava `atualizacao-cadastral` → `atualizacao-cadastral` · 4.19s
  - pergunta: sim
  - resposta: Cadastro atualizado com sucesso. Protocolo: MOCK-179318. Qualquer coisa, é só chamar por aqui. — Administração do condomínio
  - problemas: 
- **c-obra-confirma** · nível 5 · esperava `comunicar-obra` → `comunicar-obra` · 4.14s
  - pergunta: sim
  - resposta: Para registrar a comunicação da obra, ainda preciso que você me diga:  *   Qual será a descrição da pintura? (Ex: Pintura de paredes internas, pintura da fachada, etc.) *   Quais cômodos serão pintados? *   A obra será r
  - problemas: falta:MOCK-
- **c-transbordo-retem** · nível 4 · esperava `transbordo` → `transbordo` · 0.0s
  - pergunta: alguém aí?
  - resposta: 
  - problemas: 
- **c-encomenda-codigo** · nível 3 · esperava `codigo-retirada|encomendas` → `codigo-retirada` · 5.35s
  - pergunta: me passa o código da primeira
  - resposta: O código para o Pacote Amazon (17/10) é *4831*. Pode retirá-lo na portaria. Qualquer coisa, é só chamar por aqui. — Administração do condomínio
  - problemas: 
- **c-saudacao-depois-pedido** · nível 2 · esperava `encomendas` → `encomendas` · 4.98s
  - pergunta: chegou pacote pra mim?
  - resposta: Sim, Jataí. Você tem um Pacote Amazon (17/10) e um Envelope Correios (16/10) aguardando retirada.  Se precisar do código para retirar, é só me avisar.
  - problemas: 
- **c-despedida** · nível 2 · esperava `self|encomendas` → `self` · 1.68s
  - pergunta: valeu, era só isso!
  - resposta: De nada, Vanderlei!  Qualquer coisa, é só chamar por aqui. — Administração do condomínio
  - problemas: 
- **c-faq-depois-regimento** · nível 3 · esperava `regimento|faq` → `regimento` · 5.34s
  - pergunta: e pode levar cachorro lá?
  - resposta: Não, a permanência de animais na piscina é proibida, conforme o Art. 20 do regimento interno.
  - problemas: 
- **c-reserva-muda-data** · nível 5 · esperava `reserva-area-comum` → `reserva-area-comum` · 4.95s
  - pergunta: na verdade domingo, pode ser?
  - resposta: Confirma a reserva da Churrasqueira em 25 de outubro de 2026 para a unidade Jataí - Operação Predial? (sim/não)
  - problemas: 
- **c-dois-pedidos-sequencia** · nível 5 · esperava `liberacao-visitante` → `liberacao-visitante` · 9.51s
  - pergunta: beleza, e libera minha mãe amanhã às 15h
  - resposta: Para liberar sua mãe amanhã às 15h, por favor, entre em contato com a administração. Eu consigo apenas liberar visitantes para entrada imediata.
  - problemas: 

