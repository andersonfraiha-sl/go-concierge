import { Box } from '@mui/material';
import type { ReactNode } from 'react';
import { type Control, Controller, useWatch } from 'react-hook-form';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { SectionCard } from '@/shared/ui/SectionCard';
import { LIMITE_DO_RESUMO, type Textos } from './contratos';
import { ListaDeChips } from './ListaDeChips';

type Props = {
  control: Control<Textos>;
  editavel: boolean;
};

export function QuandoUsarAba({ control, editavel }: Props): ReactNode {
  const resumo = useWatch({ control, name: 'resumo' });

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <SectionCard
        titulo="Resumo"
        ajuda="Uma linha para as pessoas entenderem o que a skill faz. Aparece para as administradoras."
      >
        <CampoTexto
          control={control}
          name="resumo"
          fullWidth
          disabled={!editavel}
          helperText={`${resumo.length}/${LIMITE_DO_RESUMO}`}
          slotProps={{ htmlInput: { 'aria-label': 'Resumo', maxLength: LIMITE_DO_RESUMO } }}
        />
      </SectionCard>
      <SectionCard
        titulo="Como reconhecer o pedido"
        ajuda="Para o concierge decidir quando usar esta skill: em que situação o morador pede isto. Ex.: o morador quer reservar churrasqueira, salão ou quadra para uma data."
      >
        <CampoTexto
          control={control}
          name="quandoUsar"
          multiline
          minRows={2}
          fullWidth
          disabled={!editavel}
          slotProps={{ htmlInput: { 'aria-label': 'Como reconhecer o pedido' } }}
        />
      </SectionCard>
      <Controller
        control={control}
        name="palavrasChave"
        render={({ field }) => (
          <ListaDeChips
            titulo="Palavras-chave"
            ajuda="Palavras que ajudam a reconhecer o pedido."
            rotuloNovoItem="Nova palavra-chave"
            mensagemVazia="Nenhuma palavra-chave."
            itens={field.value}
            editavel={editavel}
            onMudar={field.onChange}
          />
        )}
      />
      <Controller
        control={control}
        name="frasesDeExemplo"
        render={({ field }) => (
          <ListaDeChips
            titulo="Frases de exemplo"
            ajuda="Frases que um morador diria para pedir isto."
            rotuloNovoItem="Nova frase de exemplo"
            mensagemVazia="Nenhuma frase de exemplo."
            itens={field.value}
            editavel={editavel}
            onMudar={field.onChange}
          />
        )}
      />
    </Box>
  );
}
