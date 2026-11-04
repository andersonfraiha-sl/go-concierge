import { Box } from '@mui/material';
import type { ReactNode } from 'react';
import { type Control, Controller, useWatch } from 'react-hook-form';
import { type EstiloDoRealce, useRealceDoCopiloto } from '@/copiloto/CopilotoProvider';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { SectionCard } from '@/shared/ui/SectionCard';
import { LIMITE_DO_RESUMO, ResumoSchema, type Textos } from './contratos';
import { ListaDeChips } from './ListaDeChips';

type Props = {
  nome: string;
  control: Control<Textos>;
  editavel: boolean;
};

/** Realce no campo de texto: o fundo do input, não o cartão branco em volta. */
const noCampo = (realce: EstiloDoRealce) => ({ '& .MuiInputBase-root': realce });
/** Realce na lista: o cartão inteiro, já que a mudança pode ser qualquer chip. */
const naLista = (realce: EstiloDoRealce) => ({ '& > .MuiPaper-root': realce });

export function QuandoUsarAba({ nome, control, editavel }: Props): ReactNode {
  const resumo = useWatch({ control, name: 'resumo' });
  const realceDoResumo = useRealceDoCopiloto(`skill:${nome}:resumo`);
  const realceDoQuandoUsar = useRealceDoCopiloto(`skill:${nome}:quandoUsar`);
  const realceDasPalavras = useRealceDoCopiloto(`skill:${nome}:palavrasChave`);
  const realceDasFrases = useRealceDoCopiloto(`skill:${nome}:frasesDeExemplo`);
  // Um resumo gravado fora da regra (texto antigo, outra ferramenta) avisa aqui em vez de passar calado.
  const foraDaRegra = ResumoSchema.safeParse(resumo).error?.issues[0]?.message;

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
          helperText={foraDaRegra ?? `${resumo.length}/${LIMITE_DO_RESUMO}`}
          sx={noCampo(realceDoResumo)}
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
          sx={noCampo(realceDoQuandoUsar)}
          slotProps={{ htmlInput: { 'aria-label': 'Como reconhecer o pedido' } }}
        />
      </SectionCard>
      <Controller
        control={control}
        name="palavrasChave"
        render={({ field }) => (
          <Box sx={naLista(realceDasPalavras)}>
            <ListaDeChips
              titulo="Palavras-chave"
              ajuda="Palavras que ajudam a reconhecer o pedido."
              rotuloNovoItem="Nova palavra-chave"
              mensagemVazia="Nenhuma palavra-chave."
              itens={field.value}
              editavel={editavel}
              onMudar={field.onChange}
            />
          </Box>
        )}
      />
      <Controller
        control={control}
        name="frasesDeExemplo"
        render={({ field }) => (
          <Box sx={naLista(realceDasFrases)}>
            <ListaDeChips
              titulo="Frases de exemplo"
              ajuda="Frases que um morador diria para pedir isto."
              rotuloNovoItem="Nova frase de exemplo"
              mensagemVazia="Nenhuma frase de exemplo."
              itens={field.value}
              editavel={editavel}
              onMudar={field.onChange}
            />
          </Box>
        )}
      />
    </Box>
  );
}
