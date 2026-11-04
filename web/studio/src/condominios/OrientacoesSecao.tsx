import { Box, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { AvisoDoCopiloto } from '@/copiloto/AvisoDoCopiloto';
import { useRealceDoCopiloto } from '@/copiloto/CopilotoProvider';
import { useEdicaoSobOServidor } from '@/copiloto/useEdicaoSobOServidor';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import { useAutosave } from '@/shared/autosave/useAutosave';
import { CampoTexto } from '@/shared/ui/CampoTexto';
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

  const sincronia = useEdicaoSobOServidor({
    servidor: geralGravada,
    local: geral,
    resetar: (valor) => reset({ geral: valor }),
    salvar: async (texto) => {
      aplicar(
        await salvarOrientacao(
          {
            condominio,
            geral: texto,
            por_caso: estado.mapeamento.orientacoes[condominio]?.por_caso ?? {},
          },
          escopo,
        ),
      );
    },
  });
  const { estado: salvamento, tentarDeNovo } = useAutosave({
    valor: geral,
    salvar: sincronia.salvar,
  });

  const orientacaoDoPadrao =
    condominio === PADRAO ? '' : (estado.mapeamento.orientacoes[PADRAO]?.geral ?? '').trim();

  return (
    <SectionCard
      titulo={TITULO}
      ajuda={AJUDA}
      acao={<SaveIndicator estado={salvamento} onTentarDeNovo={() => void tentarDeNovo()} />}
    >
      <AvisoDoCopiloto visivel={sincronia.mudouSobEdicao} onFechar={sincronia.dispensarAviso} />
      <CampoTexto
        control={control}
        name="geral"
        label={TITULO}
        multiline
        minRows={4}
        placeholder={PLACEHOLDER}
        disabled={!editavel}
        sx={{ '& .MuiInputBase-root': realce }}
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
