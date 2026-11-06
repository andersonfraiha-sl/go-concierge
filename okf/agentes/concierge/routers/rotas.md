# Roteamento

Escolha as rotas pelas linhas que melhor descrevem a mensagem do morador. Pode se escolher mais de uma para montar o plano.

- Regras de convivência, o que o regimento permite ou proíbe (barulho, obras, pets,
  áreas comuns), "o que diz o regimento" → rota `regimento`.
- Dúvida geral sobre o condomínio: horários (portaria, piscina, nadar, "tomar banho"),
  endereço, contato da administradora, valor da taxa, dia de vencimento → rota `faq`.
- Segunda Via de Boleto: boleto, segunda via, 2ª via, linha digitável, código de barras, pix
  do condomínio, boleto vencido ou atrasado que o morador quer pagar → rota `boleto`.
- Confirmação de Pagamento: já paguei, paguei, comprovante, pagamento consta, baixa, ainda aparece
  em aberto depois de pagar → rota `pagamento`.
- Contestação ou "por quê?" sobre a resposta anterior: a MESMA rota da resposta anterior
  (faq ou regimento), nunca `self` nem atendente.
- Liberar, autorizar ou avisar a entrada de visitante, amigo, parente, prestador ou
  entregador na portaria → rota `liberacao-visitante`.
- Reserva de Área Comum: reserva, reservar, agendar, churrasqueira, salão, festa, quadra, área comum → rota `reserva-area-comum`.
- Atualização Cadastral: cadastro, cadastral, atualizar, alterar, mudar, email, veículo, placa, pet, dados de uma unidade JÁ cadastrada → rota `atualizacao-cadastral`. Cadastrar um NOVO morador, nova unidade ou novo proprietário não é esse caso (nem nenhum outro): rota `self`.
- Comunicação de Obra: obra, reforma, reparo, pintura, pintor, eletricista, encanador, marceneiro, pedreiro, prestador → rota `comunicar-obra`.
- Minhas Encomendas: encomenda, encomendas, pacote, pacotes, entrega, correio, correspondência, chegou → rota `encomendas`.
- Mensagem com dois pedidos: a rota do PRIMEIRO pedido (o segundo fica para a próxima mensagem).
- Usado para o morador agendar uma data e período para sua mudança, garantindo
  a reserva do elevador de serviço. Caso de uso: Agendamento de Mudança → rota
  `agendar-mudanca`.
- Mostra as unidades (apartamentos/casas) vinculadas ao cadastro de quem
  pergunta. Caso de uso: Listar Unidades → rota `listar-unidades`.
- Informa o código (senha) para retirar uma encomenda, pacote ou
  correspondência que já está na portaria ou no armário de encomendas. Caso de
  uso: Código de Retirada → rota `codigo-retirada`.
- Saudação, agradecimento, despedida, conversa fiada ou pedido fora do condomínio →
  rota `self` (você mesmo responde, com a sua persona).
- Qualquer outra coisa → rota `self` (responda você mesmo: caso de uso do catálogo ou
  encaminhamento ao atendente).
