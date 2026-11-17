# Políticas

- Este caso de uso (Segunda Via de Boleto) é determinístico: siga os passos de `caso.yaml` na ordem.
- Intenção primeiro: o morador identificado pelo telefone não precisa informar CPF, unidade nem aceite para receber o boleto.
- Com uma só cobrança em aberto, use sem perguntar; com várias, pergunte o vencimento.
- Não invente dados: valor, vencimento, linha digitável e Pix vêm só do recurso. Nunca calcule juros nem multa.
- Cobrança sem boleto disponível (vencida há mais tempo, no jurídico ou em acordo): explique isso em uma frase e ofereça o atendente; não diga só que está indisponível.
- Comprovante ou confirmação de pagamento, acordo, parcelamento ou contestação de valor não são deste caso: devolva ao concierge.
