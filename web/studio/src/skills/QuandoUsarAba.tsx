import { Box } from '@mui/material';
import type { ComponentProps, ReactNode } from 'react';
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

export function QuandoUsarAba(props: Props): ReactNode {
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <SecaoDoResumo {...props} />
      <SecaoQuandoUsar {...props} />
      <ListaDoCopiloto {...props} campo="palavrasChave" />
      <ListaDoCopiloto {...props} campo="frasesDeExemplo" />
    </Box>
  );
}

function SecaoDoResumo({ nome, control, editavel }: Props): ReactNode {
  const resumo = useWatch({ control, name: 'resumo' });
  const realce = useRealceDoCopiloto(`skill:${nome}:resumo`);
  // Um resumo gravado fora da regra (texto antigo, outra ferramenta) avisa aqui em vez de passar calado.
  const foraDaRegra = ResumoSchema.safeParse(resumo).error?.issues[0]?.message;
  return (
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
        sx={noCampo(realce)}
        slotProps={{ htmlInput: { 'aria-label': 'Resumo', maxLength: LIMITE_DO_RESUMO } }}
      />
    </SectionCard>
  );
}

function SecaoQuandoUsar({ nome, control, editavel }: Props): ReactNode {
  const realce = useRealceDoCopiloto(`skill:${nome}:quandoUsar`);
  return (
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
        sx={noCampo(realce)}
        slotProps={{ htmlInput: { 'aria-label': 'Como reconhecer o pedido' } }}
      />
    </SectionCard>
  );
}

type CampoDeLista = 'palavrasChave' | 'frasesDeExemplo';

const TEXTOS_DAS_LISTAS: Record<
  CampoDeLista,
  Pick<ComponentProps<typeof ListaDeChips>, 'titulo' | 'ajuda' | 'rotuloNovoItem' | 'mensagemVazia'>
> = {
  palavrasChave: {
    titulo: 'Palavras-chave',
    ajuda: 'Palavras que ajudam a reconhecer o pedido.',
    rotuloNovoItem: 'Nova palavra-chave',
    mensagemVazia: 'Nenhuma palavra-chave.',
  },
  frasesDeExemplo: {
    titulo: 'Frases de exemplo',
    ajuda: 'Frases que um morador diria para pedir isto.',
    rotuloNovoItem: 'Nova frase de exemplo',
    mensagemVazia: 'Nenhuma frase de exemplo.',
  },
};

function ListaDoCopiloto({ nome, control, editavel, campo }: Props & { campo: CampoDeLista }) {
  const realce = useRealceDoCopiloto(`skill:${nome}:${campo}`);
  return (
    <Controller
      control={control}
      name={campo}
      render={({ field }) => (
        <Box sx={naLista(realce)}>
          <ListaDeChips
            {...TEXTOS_DAS_LISTAS[campo]}
            itens={field.value}
            editavel={editavel}
            onMudar={field.onChange}
          />
        </Box>
      )}
    />
  );
}
