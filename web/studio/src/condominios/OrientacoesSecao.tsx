import { Box, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { useForm, useWatch } from 'react-hook-form';
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

export function OrientacoesSecao({ estado, condominio, editavel }: Props): ReactNode {
  const escopo = useEscopo() ?? null;
  const aplicar = useAplicarMapeamento();
  const geralGravada = estado.mapeamento.orientacoes[condominio]?.geral ?? '';
  const { control, reset } = useForm<{ geral: string }>({
    defaultValues: { geral: geralGravada },
  });
  const geral = useWatch({ control, name: 'geral' });
  const realce = useRealceDoCopiloto(`condominio:${condominio}:orientacao`);

  // A hora da última gravação do mapa serve de versão: uma leitura mais velha não volta o campo.
  const sincronia = useEdicaoSobOServidor({
    servidor: geralGravada,
    alvos: [`condominio:${condominio}:orientacao`],
    versao: estado.mapeamento.atualizado_em ?? null,
    local: geral,
    resetar: (valor) => reset({ geral: valor }),
    salvar: async (texto) => {
      const resposta = await salvarOrientacao(
        {
          condominio,
          geral: texto,
          por_caso: estado.mapeamento.orientacoes[condominio]?.por_caso ?? {},
        },
        escopo,
      );
      aplicar(resposta);
      return resposta.mapeamento.atualizado_em ?? null;
    },
  });
  const { estado: salvamento, tentarDeNovo } = useAutosave({
    valor: geral,
    salvar: sincronia.salvar,
    bloqueio: sincronia.bloqueio,
  });

  const orientacaoDoPadrao =
    condominio === PADRAO ? '' : (estado.mapeamento.orientacoes[PADRAO]?.geral ?? '').trim();

  return (
    <SectionCard
      titulo={TITULO}
      ajuda={AJUDA}
      acao={<SaveIndicator estado={salvamento} onTentarDeNovo={() => void tentarDeNovo()} />}
    >
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
      {orientacaoDoPadrao !== '' && (
        <Box>
          <Typography variant="body2" sx={{ fontWeight: 600 }}>
            Também vale o do Padrão:
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {orientacaoDoPadrao}
          </Typography>
        </Box>
      )}
    </SectionCard>
  );
}
