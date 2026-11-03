import { Box, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { useForm, useWatch } from 'react-hook-form';
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

const AJUDA =
  'Escreva como você explicaria a um atendente novo: horários, nomes, regras locais, o que não fazer. O concierge lê isso em toda conversa deste condomínio.';
const PLACEHOLDER =
  'Ex.: A piscina fecha às 22h. O síndico é o sr. Paulo (não passe o telefone dele). Não aceitamos mudança aos domingos. Entregadores só até as 21h.';

export function OrientacoesSecao({ estado, condominio, editavel }: Props): ReactNode {
  const escopo = useEscopo() ?? null;
  const aplicar = useAplicarMapeamento();
  const { control } = useForm<{ geral: string }>({
    defaultValues: { geral: estado.mapeamento.orientacoes[condominio]?.geral ?? '' },
  });

  const { estado: salvamento, tentarDeNovo } = useAutosave({
    valor: useWatch({ control, name: 'geral' }),
    salvar: async (geral) => {
      aplicar(
        await salvarOrientacao(
          {
            condominio,
            geral,
            por_caso: estado.mapeamento.orientacoes[condominio]?.por_caso ?? {},
          },
          escopo,
        ),
      );
    },
  });

  const orientacaoDoPadrao =
    condominio === PADRAO ? '' : (estado.mapeamento.orientacoes[PADRAO]?.geral ?? '').trim();

  return (
    <SectionCard
      titulo="Orientações gerais"
      ajuda={AJUDA}
      acao={<SaveIndicator estado={salvamento} onTentarDeNovo={() => void tentarDeNovo()} />}
    >
      <CampoTexto
        control={control}
        name="geral"
        label="Orientações gerais"
        multiline
        minRows={4}
        placeholder={PLACEHOLDER}
        disabled={!editavel}
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
