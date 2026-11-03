import { Box, Button, Drawer } from '@mui/material';
import type { ReactNode } from 'react';
import { Link } from 'react-router';
import { pode, useEu, usePodeEditar } from '@/acesso/eu';
import { AjustesDaSkill } from './AjustesDaSkill';
import { Cabecalho } from './Cabecalho';
import { ComoReconhece } from './ComoReconhece';
import { type Caso, type Estado, habilitacaoDe, PADRAO, salvarHabilitacao } from './contratos';
import { origemDe } from './listaDeSkills';
import { OrientacaoDaSkill } from './OrientacaoDaSkill';
import { QuemPodeUsar } from './QuemPodeUsar';
import { useExecutar } from './useExecutar';

type Props = {
  estado: Estado;
  condominio: string;
  caso: Caso;
  titulo: string;
  editavel: boolean;
  onFechar: () => void;
};

export function SkillDoCondominioDrawer({
  estado,
  condominio,
  caso,
  titulo,
  editavel,
  onFechar,
}: Props): ReactNode {
  const { data: eu } = useEu();
  const { executar, gravando } = useExecutar();
  const podeVerTextoDaSkill = usePodeEditar('skills.editar_textos');

  const encontrado = estado.mapeamento.condominios.find((c) => c.id === condominio);
  const nomeDoCondominio =
    condominio === PADRAO ? 'Padrão (todos)' : (encontrado?.nome ?? condominio);

  const { habilitacao, propria } = habilitacaoDe(estado.mapeamento, condominio, caso.nome);
  const travado = !editavel || gravando;

  const gravarHabilitacao = (habilitado: boolean, papeis: string[]): Promise<boolean> =>
    executar((escopo) =>
      salvarHabilitacao({ condominio, caso: caso.nome, habilitado, papeis, limpar: false }, escopo),
    );

  const voltarAoPadrao = (): Promise<boolean> =>
    executar((escopo) =>
      salvarHabilitacao(
        { condominio, caso: caso.nome, habilitado: true, papeis: [], limpar: true },
        escopo,
      ),
    );

  return (
    <Drawer
      anchor="right"
      open
      onClose={onFechar}
      slotProps={{
        paper: {
          sx: { width: 'min(420px, 100vw)' },
          role: 'dialog',
          'aria-label': `${titulo} · ${nomeDoCondominio}`,
        },
      }}
    >
      <Box sx={{ p: 2, display: 'flex', flexDirection: 'column', gap: 2 }}>
        <Cabecalho
          caso={caso}
          titulo={titulo}
          habilitada={habilitacao.habilitado}
          origem={origemDe(estado.mapeamento, condominio, caso.nome)}
          travado={travado}
          onMudarHabilitada={(ligado) => void gravarHabilitacao(ligado, habilitacao.papeis)}
        />
        <QuemPodeUsar
          papeis={estado.papeis}
          escolhidos={habilitacao.papeis}
          travado={travado}
          onMudar={(papeis) => void gravarHabilitacao(habilitacao.habilitado, papeis)}
        />
        <OrientacaoDaSkill
          estado={estado}
          condominio={condominio}
          caso={caso}
          editavel={editavel}
        />
        <AjustesDaSkill estado={estado} condominio={condominio} caso={caso} editavel={editavel} />
        {podeVerTextoDaSkill && <ComoReconhece descricao={caso.descricao} />}
        <Box sx={{ display: 'flex', gap: 1 }}>
          {eu && pode(eu, 'skills.ler') && (
            <Button
              fullWidth
              component={Link}
              to={`/skills/${encodeURIComponent(caso.nome)}?aba=fluxo`}
            >
              Ver o fluxo
            </Button>
          )}
          {propria && editavel && (
            <Button fullWidth onClick={() => void voltarAoPadrao()} disabled={gravando}>
              Voltar ao Padrão
            </Button>
          )}
        </Box>
      </Box>
    </Drawer>
  );
}
