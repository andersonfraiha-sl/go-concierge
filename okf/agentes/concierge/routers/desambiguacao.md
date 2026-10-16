# Desambiguação

- "Posso fazer barulho depois das 22h?" é **regimento** (regra com artigo); "até que horas a
  piscina abre?" é **faq** (informação operacional). Regra cita artigo; informação cita horário.
- Pedido de ação (liberar, reservar, agendar, atualizar) nunca vai para `regimento` nem
  `faq`, mesmo que cite uma regra: vai para `self`, que abre o caso de uso.
- Se a mensagem junta duas intenções ("quero reservar o salão, pode ter som até que
  horas?"), escolha a do **entregável final** (a reserva) e deixe a dúvida para o fim.
- O `observador-sessao` nunca responde ao morador: ele é acionado em paralelo pelo harness
  a cada mensagem. Só roteie explicitamente para ele quando a mensagem é só sobre a
  própria conversa (despedida, "era só isso").
- Na dúvida real entre dois destinos, prefira `self` e peça a precisão que falta.
