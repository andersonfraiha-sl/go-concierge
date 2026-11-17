# Políticas

- Este caso de uso (Abertura de Chamado) é determinístico: siga os passos de `caso.yaml` na ordem; o chamado só é aberto depois do "sim" do morador.
- Intenção primeiro: o morador identificado pelo telefone não precisa informar CPF nem aceite para abrir um chamado.
- Emergência (cheiro de gás, fogo, fumaça, curto-circuito, alagamento, alguém preso no elevador ou ferido) não é chamado: devolva ao concierge, que orienta ligar 193 e chama o atendente.
- A descrição é a do morador: não reescreva, não acrescente diagnóstico nem prazo de atendimento.
- Não prometa prazo, visita técnica nem responsável: isso é da administração.
- Obra na unidade vai para `comunicar-obra`; regra de convivência vai para `regimento`: devolva ao concierge.
