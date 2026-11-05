import { Box, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import type { Ferramenta } from '@/catalogo/contratos';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import { SectionCard } from '@/shared/ui/SectionCard';
import { CamposDaFerramenta } from './CamposDaFerramenta';
import { type Estado, ferramentasGerais } from './contratos';
import { PreencherComIA } from './PreencherComIA';
import { type AjustesEmEdicao, useAjustes } from './useAjustes';

type Props = { estado: Estado; condominio: string; editavel: boolean };

export function AjustesSecao({ estado, condominio, editavel }: Props): ReactNode {
  const ferramentas = ferramentasGerais(estado);
  const ajustes = useAjustes({ estado, condominio, ferramentas });
  const { salvamento, tentarDeNovo } = ajustes;

  return (
    <SectionCard
      titulo="Ajustes gerais"
      ajuda="Prazos, horários e códigos que não são de uma skill só. Campo vazio vale o padrão."
      acao={<SaveIndicator estado={salvamento} onTentarDeNovo={() => void tentarDeNovo()} />}
    >
      <CorpoDosAjustes
        condominio={condominio}
        ferramentas={ferramentas}
        ajustes={ajustes}
        editavel={editavel}
      />
    </SectionCard>
  );
}

type CorpoProps = {
  condominio: string;
  ferramentas: Ferramenta[];
  ajustes: AjustesEmEdicao;
  editavel: boolean;
};

function CorpoDosAjustes({ condominio, ferramentas, ajustes, editavel }: CorpoProps): ReactNode {
  if (ferramentas.length === 0) return <SemAjustesGerais />;
  const comum = { ajustes, editavel };

  return (
    <>
      {editavel && (
        <PreencherComIA
          condominio={condominio}
          ferramentas={ferramentas}
          valores={ajustes.valores}
          onPreencher={ajustes.mudar}
        />
      )}
      {ferramentas.map((ferramenta) => (
        <FerramentaGeral key={ferramenta.name} ferramenta={ferramenta} {...comum} />
      ))}
    </>
  );
}

function SemAjustesGerais(): ReactNode {
  return (
    <Typography color="textSecondary">
      Os ajustes de cada skill ficam na própria skill, na lista acima.
    </Typography>
  );
}

type FerramentaProps = { ferramenta: Ferramenta; ajustes: AjustesEmEdicao; editavel: boolean };

function FerramentaGeral({ ferramenta, ajustes, editavel }: FerramentaProps): ReactNode {
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
      <Typography variant="subtitle2">{ferramenta.titulo ?? ferramenta.name}</Typography>
      <CamposDaFerramenta ferramenta={ferramenta} ajustes={ajustes} editavel={editavel} />
    </Box>
  );
}
