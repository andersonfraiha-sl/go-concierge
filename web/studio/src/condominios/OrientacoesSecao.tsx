import { Box, TextField } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import { useAutosave } from '@/shared/autosave/useAutosave';
import { SectionCard } from '@/shared/ui/SectionCard';
import {
  type Estado,
  type Orientacao,
  salvarOrientacao,
  useAplicarMapeamento,
  useEscopo,
} from './contratos';

type Props = {
  estado: Estado;
  condominio: string;
  titulos: ReadonlyMap<string, string>;
  editavel: boolean;
};

const SEM_ORIENTACAO: Orientacao = { geral: '', por_caso: {} };

const semTextosVazios = (porCaso: Record<string, string>): Record<string, string> =>
  Object.fromEntries(Object.entries(porCaso).filter(([, texto]) => texto.trim() !== ''));

export function OrientacoesSecao({ estado, condominio, titulos, editavel }: Props): ReactNode {
  const escopo = useEscopo() ?? null;
  const aplicar = useAplicarMapeamento();
  const [orientacao, setOrientacao] = useState<Orientacao>(
    estado.mapeamento.orientacoes[condominio] ?? SEM_ORIENTACAO,
  );

  const { estado: salvamento, tentarDeNovo } = useAutosave({
    valor: orientacao,
    salvar: async ({ geral, por_caso }) => {
      aplicar(
        await salvarOrientacao({ condominio, geral, por_caso: semTextosVazios(por_caso) }, escopo),
      );
    },
  });

  const mudarCaso = (caso: string, texto: string): void =>
    setOrientacao((atual) => ({ ...atual, por_caso: { ...atual.por_caso, [caso]: texto } }));

  return (
    <SectionCard
      titulo="Orientações"
      ajuda="O que a assistente precisa saber deste condomínio."
      acao={<SaveIndicator estado={salvamento} onTentarDeNovo={() => void tentarDeNovo()} />}
    >
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
        <TextField
          label="Para todas as skills"
          multiline
          minRows={3}
          disabled={!editavel}
          value={orientacao.geral}
          onChange={(e) => setOrientacao((atual) => ({ ...atual, geral: e.target.value }))}
        />
        {estado.casos.map((caso) => (
          <TextField
            key={caso.nome}
            label={titulos.get(caso.nome) ?? caso.titulo}
            multiline
            minRows={2}
            disabled={!editavel}
            value={orientacao.por_caso[caso.nome] ?? ''}
            onChange={(e) => mudarCaso(caso.nome, e.target.value)}
          />
        ))}
      </Box>
    </SectionCard>
  );
}
