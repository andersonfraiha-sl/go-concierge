| Métrica | Rodada 1 | Rodada 2 |
|---|---|---|
| casos | 500 | 500 |
| ok | 451 (90.2%) | 478 (95.6%) |
| rota errada | 46 (9.2%) | 18 (3.6%) |
| resposta vazia | 3 | 0 |
| turno-erro | 2 | 0 |
| tool-error | 3 | 0 |
| vazamento | 0 | 0 |
| timeout | 0 | 0 |
| p50 (s) | 6.2 | 5.4 |
| p90 (s) | 9.9 | 8.7 |
| p95 (s) | 13.6 | 10.3 |
| máx (s) | 34.7 | 33.9 |
| média (s) | 6.8 | 5.6 |
| chamadas/resposta | 3.6 | 3.6 |
| tokens/resposta | 6697 | 6913 |

| Tipo | Casos | OK r1 | OK r2 | p50 r1 | p50 r2 |
|---|---|---|---|---|---|
| acao | 133 | 98% | 95% | 6.7s | 5.8s |
| acao-fluxo | 9 | 78% | 78% | 4.9s | 4.2s |
| complexo | 38 | 68% | 82% | 12.8s | 7.4s |
| complexo-fluxo | 1 | 100% | 100% | 5.8s | 9.5s |
| consulta | 60 | 88% | 100% | 6.6s | 5.8s |
| consulta-fluxo | 1 | 100% | 100% | 5.8s | 5.3s |
| conteudo | 128 | 96% | 97% | 6.6s | 6.2s |
| conteudo-fluxo | 1 | 100% | 100% | 6.7s | 5.3s |
| fora | 39 | 87% | 95% | 3.7s | 3.3s |
| social | 52 | 98% | 100% | 2.9s | 2.5s |
| social-fluxo | 2 | 100% | 100% | 5.4s | 3.3s |
| transbordo | 35 | 60% | 97% | 3.7s | 3.3s |
| transbordo-fluxo | 1 | 100% | 100% | 0.0s | 0.0s |

Melhoraram: 37 · Pioraram: 10 · Continuam com problema: 12

Pioraram:
- complexo-170 · qual o horário de silêncio? e também dá pra agendar o salão de festas dia 12/11? → transbordo · rota:transbordo!=regimento|reserva-area-comum
- complexo-172 · o que o regimento diz sobre obras? e também me reserva a churrasqueira pra amanh → transbordo · rota:transbordo!=regimento|reserva-area-comum
- armadilha-177 · depois de amanhã chega um pacote, avisa a portaria → transbordo · rota:transbordo!=encomendas|liberacao-visitante|self
- armadilha-187 · meu apartamento é o 101 ou o 102? esqueci → listar-unidades · resposta-de-erro
- comunicar-obra-192 · oi, tudo bem? vou trocar o piso, preciso avisar? → regimento · rota:regimento!=comunicar-obra
- reserva-area-comum-195 · reserva a quadra pra domingo de manhã kkk → transbordo · rota:transbordo!=reserva-area-comum, resposta-de-erro
- atualizacao-cadastral-215 · preciso corrigir meu nome no cdastro → transbordo · rota:transbordo!=atualizacao-cadastral
- atualizacao-cadastral-220 · preciso corrigir meu nome no cadastro (pra hoje) → transbordo · rota:transbordo!=atualizacao-cadastral
- atualizacao-cadastral-254 · quero incluir minha esposa no cadastro urgente → transbordo · rota:transbordo!=atualizacao-cadastral
- reserva-area-comum-388 · olá, aqui é o morador do 101. queria bater um churras no próximo sábado → reserva-area-comum · resposta-de-erro
