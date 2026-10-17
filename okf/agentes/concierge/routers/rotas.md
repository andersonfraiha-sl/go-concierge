# Roteamento

Escolha a rota pela linha que melhor descreve a mensagem do morador.

- Regras de convivência, o que o regimento permite ou proíbe (barulho, obras, pets,
  áreas comuns), "o que diz o regimento" → rota `regimento`.
- Dúvida geral sobre o condomínio: horários (portaria, piscina), endereço, contato da
  administradora, taxa, boleto → rota `faq`.
- Liberar, autorizar ou avisar a entrada de visitante, amigo, parente, prestador ou
  entregador na portaria → rota `liberacao-visitante`.
- Saudação, agradecimento, despedida, conversa fiada ou pedido fora do condomínio →
  rota `self` (você mesmo responde, com a sua persona).
- Qualquer outra coisa → rota `self` (responda você mesmo: caso de uso do catálogo ou
  encaminhamento ao atendente).
