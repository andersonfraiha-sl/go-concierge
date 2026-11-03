import { Box, Button, Chip, Switch, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { SectionCard } from '@/shared/ui/SectionCard';
import { habilitacaoDe, salvarHabilitacao, type Caso, type Estado } from './contratos';
import { useExecutar } from './useExecutar';

const ROTULO_PAPEL: Record<string, string> = {
  morador: 'morador',
  proprietario: 'proprietário',
  sindico: 'síndico',
  publico: 'público',
};

const rotuloDoPapel = (papel: string): string => ROTULO_PAPEL[papel] ?? papel;

type Props = {
  estado: Estado;
  condominio: string;
  titulos: ReadonlyMap<string, string>;
  editavel: boolean;
};

export function SkillsSecao({ estado, condominio, titulos, editavel }: Props): ReactNode {
  const { executar, gravando } = useExecutar();

  const gravar = (caso: Caso, habilitado: boolean, papeis: string[]): Promise<boolean> =>
    executar((escopo) =>
      salvarHabilitacao({ condominio, caso: caso.nome, habilitado, papeis, limpar: false }, escopo),
    );

  const usarOPadrao = (caso: Caso): Promise<boolean> =>
    executar((escopo) =>
      salvarHabilitacao({ condominio, caso: caso.nome, habilitado: true, papeis: [], limpar: true }, escopo),
    );

  return (
    <SectionCard titulo="Skills neste condomínio" ajuda="Ligue o que a assistente pode fazer aqui.">
      {estado.casos.length === 0 && (
        <Typography color="text.secondary">A assistente ainda não tem skills.</Typography>
      )}
      {estado.casos.map((caso) => {
        const titulo = titulos.get(caso.nome) ?? caso.titulo;
        const { habilitacao, propria } = habilitacaoDe(estado.mapeamento, condominio, caso.nome);
        return (
          <Box key={caso.nome} sx={{ display: 'flex', flexDirection: 'column', gap: 0.5, opacity: habilitacao.habilitado ? 1 : 0.7 }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <Switch
                checked={habilitacao.habilitado}
                disabled={!editavel || gravando}
                onChange={(_, ligado) => void gravar(caso, ligado, habilitacao.papeis)}
                slotProps={{ input: { 'aria-label': `${titulo} neste condomínio` } }}
              />
              <Box sx={{ flex: 1 }}>
                <Typography>{titulo}</Typography>
                <Typography variant="caption" color="text.secondary">
                  {caso.descricao}
                </Typography>
              </Box>
              {propria && editavel && (
                <Button size="small" disabled={gravando} aria-label={`Usar o padrão: ${titulo}`} onClick={() => void usarOPadrao(caso)}>
                  Usar o padrão
                </Button>
              )}
            </Box>
            <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap', pl: 7 }}>
              {estado.papeis.map((papel) => {
                const marcado = habilitacao.papeis.includes(papel);
                const proximos = marcado ? habilitacao.papeis.filter((p) => p !== papel) : [...habilitacao.papeis, papel];
                return (
                  <Chip
                    key={papel}
                    size="small"
                    label={rotuloDoPapel(papel)}
                    color={marcado ? 'primary' : 'default'}
                    variant={marcado ? 'filled' : 'outlined'}
                    clickable={editavel}
                    disabled={!editavel || gravando}
                    aria-label={`${rotuloDoPapel(papel)} em ${titulo}`}
                    onClick={() => void gravar(caso, habilitacao.habilitado, proximos)}
                  />
                );
              })}
              {habilitacao.papeis.length === 0 && (
                <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                  Sem papel marcado: vale para todos.
                </Typography>
              )}
            </Box>
          </Box>
        );
      })}
    </SectionCard>
  );
}
