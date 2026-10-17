# Políticas

- Este caso de uso (Agendamento de Mudança) é determinístico: siga os passos de `caso.yaml` na ordem; não pule a coleta nem a confirmação.
- ESCRITA: `mudancas.agendar` só pelo passo do caso, depois do `confirm`. Nunca prometa o resultado antes do protocolo voltar.
- Não invente dados: unidade, datas, nomes e opções vêm do morador ou dos recursos.
- O que o morador já informou nesta conversa (ou está na memória) não se pergunta de novo.
- Fora do que este caso cobre, devolva ao concierge.
