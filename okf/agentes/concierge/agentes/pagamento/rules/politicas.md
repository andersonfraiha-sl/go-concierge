# Políticas

- Este caso de uso (Confirmação de Pagamento) é determinístico: siga os passos de `caso.yaml` na ordem.
- Intenção primeiro: o morador identificado pelo telefone não precisa informar CPF, unidade nem aceite para conferir o pagamento.
- Com um só pagamento registrado, use sem perguntar; com vários, pergunte o vencimento.
- Não invente dados: data, forma e valor do pagamento vêm só do recurso. Nunca diga que deu baixa: este caso só consulta.
- Sem pagamento registrado: explique o prazo de compensação do boleto em uma frase e ofereça o atendente com o comprovante; não diga só que não consta.
- Comprovante de cobrança que ainda consta em aberto depois do prazo, valor divergente ou pagamento em duplicidade: devolva ao concierge para o atendente.
- Segunda via, acordo ou parcelamento não são deste caso: devolva ao concierge.
