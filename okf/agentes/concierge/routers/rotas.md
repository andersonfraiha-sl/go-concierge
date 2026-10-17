# Roteamento

Escolha a rota pela linha que melhor descreve a mensagem do morador.

- Regras de convivência, o que o regimento permite ou proíbe (barulho, obras, pets,
  áreas comuns), "o que diz o regimento" → rota `regimento`.
- Dúvida geral sobre o condomínio: horários (portaria, piscina), endereço, contato da
  administradora, taxa, boleto → rota `faq`.
- Liberar, autorizar ou avisar a entrada de visitante, amigo, parente, prestador ou
  entregador na portaria → rota `liberacao-visitante`.
- Listar Unidades: unidade, unidades, apartamento, apartamentos, apto, imóvel, imóveis, vinculadas → rota `listar-unidades`.
- Reserva de Área Comum: reserva, reservar, agendar, churrasqueira, salão, festa, quadra, área comum → rota `reserva-area-comum`.
- Atualização Cadastral: cadastro, cadastral, atualizar, alterar, mudar, email, veículo, placa, pet, dados → rota `atualizacao-cadastral`.
- Comunicação de Obra: obra, reforma, reparo, pintura, pintor, eletricista, encanador, marceneiro, pedreiro, prestador → rota `comunicar-obra`.
- Minhas Encomendas: encomenda, encomendas, pacote, pacotes, entrega, correio, correspondência, chegou → rota `encomendas`.
- Código de Retirada: código, codigo, retirada, retirar, senha, encomenda, pacote → rota `codigo-retirada`.
- Usado para o morador agendar uma data e período para sua mudança, garantindo
  a reserva do elevador de serviço. Caso de uso: Agendamento de Mudança → rota
  `agendar-mudanca`.
- Saudação, agradecimento, despedida, conversa fiada ou pedido fora do condomínio →
  rota `self` (você mesmo responde, com a sua persona).
- Qualquer outra coisa → rota `self` (responda você mesmo: caso de uso do catálogo ou
  encaminhamento ao atendente).
