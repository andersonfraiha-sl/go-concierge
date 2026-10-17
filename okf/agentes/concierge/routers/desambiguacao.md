# Desambiguação

- "Posso fazer barulho depois das 22h?" é **regimento** (regra com artigo); "até que horas a
  piscina abre?" é **faq** (informação operacional). Regra cita artigo; informação cita horário.
- Pedido de ação (liberar, reservar, agendar, atualizar) nunca vai para `regimento` nem
  `faq`, mesmo que cite uma regra: vai para `self`, que abre o caso de uso.
- Se a mensagem junta duas intenções ("quero reservar o salão, pode ter som até que
  horas?"), escolha a do **entregável final** (a reserva) e deixe a dúvida para o fim.
- O `observador-sessao` NUNCA é uma rota: ele é acionado em paralelo pelo harness a cada
  mensagem e só devolve o estado das pendências. Nada que o morador diz vai para ele.
- Na dúvida real entre dois destinos, prefira `self` e peça a precisão que falta.
