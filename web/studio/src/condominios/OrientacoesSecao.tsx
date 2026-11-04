import { Box, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { type Control, type UseFormReset, useForm, useWatch } from 'react-hook-form';
import { AvisoDeConflito } from '@/copiloto/AvisoDeConflito';
import { useRealceDoCopiloto } from '@/copiloto/CopilotoProvider';
import { useEdicaoSobOServidor } from '@/copiloto/useEdicaoSobOServidor';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import { useAutosave } from '@/shared/autosave/useAutosave';
import { CampoRico } from '@/shared/editor/CampoRico';
import { SectionCard } from '@/shared/ui/SectionCard';
import {
  type Estado,
  PADRAO,
  salvarOrientacao,
  useAplicarMapeamento,
  useEscopo,
} from './contratos';

type Props = { estado: Estado; condominio: string; editavel: boolean };

const TITULO = 'Orientações para este condomínio';
const AJUDA = 'Vale para todas as conversas daqui. Escreva como falaria com um porteiro novo.';
const PLACEHOLDER =
  'Ex.: A piscina fecha às 22h. O síndico é o sr. Paulo (não passe o telefone dele). Não aceitamos mudança aos domingos. Entregadores só até as 21h.';

type Sincronia = ReturnType<typeof useEdicaoSobOServidor<string, string | null>>;

function useGravarGeral(
  estado: Estado,
  condominio: string,
): (texto: string) => Promise<string | null> {
  const escopo = useEscopo() ?? null;
  const aplicar = useAplicarMapeamento();
  return async (texto) => {
    const porCaso = estado.mapeamento.orientacoes[condominio]?.por_caso ?? {};
    const resposta = await salvarOrientacao(
      { condominio, geral: texto, por_caso: porCaso },
      escopo,
    );
    aplicar(resposta);
    return resposta.mapeamento.atualizado_em ?? null;
  };
}

type EntradaDaSincronia = {
  estado: Estado;
  condominio: string;
  geral: string;
  reset: UseFormReset<{ geral: string }>;
};

function useSincroniaDoGeral({ estado, condominio, geral, reset }: EntradaDaSincronia): Sincronia {
  const gravar = useGravarGeral(estado, condominio);
  // A hora da última gravação do mapa serve de versão: uma leitura mais velha não volta o campo.
  return useEdicaoSobOServidor({
    servidor: estado.mapeamento.orientacoes[condominio]?.geral ?? '',
    alvos: [`condominio:${condominio}:orientacao`],
    versao: estado.mapeamento.atualizado_em ?? null,
    local: geral,
    resetar: (valor) => reset({ geral: valor }),
    salvar: gravar,
  });
}

function useOrientacaoGeral(estado: Estado, condominio: string) {
  const { control, reset } = useForm<{ geral: string }>({
    defaultValues: { geral: estado.mapeamento.orientacoes[condominio]?.geral ?? '' },
  });
  const geral = useWatch({ control, name: 'geral' });
  const sincronia = useSincroniaDoGeral({ estado, condominio, geral, reset });
  const { salvar, bloqueio } = sincronia;
  const { estado: salvamento, tentarDeNovo } = useAutosave({ valor: geral, salvar, bloqueio });
  return { control, sincronia, salvamento, tentarDeNovo };
}

export function OrientacoesSecao({ estado, condominio, editavel }: Props): ReactNode {
  const { control, sincronia, salvamento, tentarDeNovo } = useOrientacaoGeral(estado, condominio);

  return (
    <SectionCard
      titulo={TITULO}
      ajuda={AJUDA}
      acao={<SaveIndicator estado={salvamento} onTentarDeNovo={() => void tentarDeNovo()} />}
    >
      <EditorDaOrientacao
        control={control}
        sincronia={sincronia}
        editavel={editavel}
        condominio={condominio}
      />
      <OrientacaoDoPadrao estado={estado} condominio={condominio} />
    </SectionCard>
  );
}

type EditorProps = {
  control: Control<{ geral: string }>;
  sincronia: Sincronia;
  editavel: boolean;
  condominio: string;
};

function EditorDaOrientacao({ control, sincronia, editavel, condominio }: EditorProps): ReactNode {
  const realce = useRealceDoCopiloto(`condominio:${condominio}:orientacao`);
  return (
    <>
      <AvisoDeConflito
        conflito={sincronia.conflito}
        onManterOMeu={sincronia.manterOMeu}
        onUsarONovo={sincronia.usarONovo}
      />
      <CampoRico
        control={control}
        name="geral"
        rotulo={TITULO}
        editavel={editavel}
        placeholder={PLACEHOLDER}
        realce={realce}
      />
    </>
  );
}

function OrientacaoDoPadrao({ estado, condominio }: Omit<Props, 'editavel'>): ReactNode {
  const doPadrao = (estado.mapeamento.orientacoes[PADRAO]?.geral ?? '').trim();
  if (condominio === PADRAO || doPadrao === '') return null;

  return (
    <Box>
      <Typography variant="body2" sx={{ fontWeight: 600 }}>
        Também vale o do Padrão:
      </Typography>
      <Typography variant="body2" color="text.secondary">
        {doPadrao}
      </Typography>
    </Box>
  );
}
