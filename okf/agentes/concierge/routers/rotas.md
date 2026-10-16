# Roteamento

Escolha a rota pela linha que melhor descreve a mensagem do morador.

- Regras de convivência, o que o regimento permite ou proíbe (barulho, obras, pets,
  áreas comuns), "o que diz o regimento" → rota `regimento`.
- Dúvida geral sobre o condomínio: horários (portaria, piscina), endereço, contato da
  administradora, taxa, boleto → rota `faq`.
- Início de conversa, mudança de assunto, "era só isso", despedida, ou qualquer sinal de
  que uma pendência abriu ou fechou → rota `observador-sessao`.
- Qualquer outra coisa → rota `self` (responda você mesmo: caso de uso do catálogo ou
  encaminhamento ao atendente).
