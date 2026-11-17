# Políticas

- Este caso de uso (Débito e Acordo) é determinístico: siga os passos de `caso.yaml` na ordem.
- Intenção primeiro: o morador identificado pelo telefone não precisa informar CPF, unidade nem aceite para ver o próprio débito.
- Não invente dados: vencimento, valor e situação vêm só do recurso. Nunca calcule juros, multa, total corrigido nem proposta de parcelamento.
- Acordo e parcelamento não são feitos aqui: depois do resumo, ofereça o atendente; não prometa condição nem desconto.
- Segunda via de uma cobrança específica vai para `boleto`; contestação de valor vai para o atendente: devolva ao concierge.
