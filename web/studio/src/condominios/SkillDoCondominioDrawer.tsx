import { Box, Button, Drawer } from '@mui/material';
import type { ReactNode } from 'react';
import { Link } from 'react-router';
import { pode, useEu, usePodeEditar } from '@/acesso/eu';
import { GAVETA } from '@/theme/tokens';
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

function nomeDoCondominio(estado: Estado, condominio: string): string {
  if (condominio === PADRAO) return 'Padrão (todos)';
  return estado.mapeamento.condominios.find((c) => c.id === condominio)?.nome ?? condominio;
}

// O desenho aprovado mostra o painel sem véu escuro: o conteúdo principal continua visível ao lado.
const slotsDoDrawer = (rotulo: string) => ({
  paper: {
    sx: { width: GAVETA.padrao, borderLeft: 1, borderColor: 'border.default' },
    role: 'dialog',
    'aria-label': rotulo,
  },
  backdrop: { sx: { bgcolor: 'transparent' } },
});

export function SkillDoCondominioDrawer(props: Props): ReactNode {
  const { estado, condominio, titulo, onFechar } = props;
  const rotulo = `${titulo} · ${nomeDoCondominio(estado, condominio)}`;

  return (
    <Drawer anchor="right" open onClose={onFechar} slotProps={slotsDoDrawer(rotulo)}>
      <ConteudoDoDrawer {...props} />
    </Drawer>
  );
}

function ConteudoDoDrawer(props: Props): ReactNode {
  const { estado, condominio, caso, titulo, editavel, onFechar } = props;
  const { cabecalho, quem, rodape } = useHabilitacaoDaSkill(props);
  const podeVerTextoDaSkill = usePodeEditar('skills.editar_textos');
  const alvo = { estado, condominio, caso, editavel };

  return (
    <Box sx={{ p: 2, display: 'flex', flexDirection: 'column', gap: 2 }}>
      <Cabecalho caso={caso} titulo={titulo} onFechar={onFechar} {...cabecalho} />
      <QuemPodeUsar {...quem} />
      <OrientacaoDaSkill {...alvo} />
      <AjustesDaSkill {...alvo} />
      {podeVerTextoDaSkill && <ComoReconhece descricao={caso.descricao} />}
      <RodapeDoDrawer caso={caso} {...rodape} />
    </Box>
  );
}

function useGravarHabilitacao(condominio: string, caso: string) {
  const { executar, gravando } = useExecutar();
  const gravar = (habilitado: boolean, papeis: string[]): Promise<boolean> =>
    executar((escopo) =>
      salvarHabilitacao({ condominio, caso, habilitado, papeis, limpar: false }, escopo),
    );
  const voltarAoPadrao = (): Promise<boolean> =>
    executar((escopo) =>
      salvarHabilitacao({ condominio, caso, habilitado: true, papeis: [], limpar: true }, escopo),
    );

  return { gravar, voltarAoPadrao, gravando };
}

function useHabilitacaoDaSkill({ estado, condominio, caso, editavel }: Props) {
  const { gravar, voltarAoPadrao, gravando } = useGravarHabilitacao(condominio, caso.nome);
  const { habilitacao, propria } = habilitacaoDe(estado.mapeamento, condominio, caso.nome);
  const travado = !editavel || gravando;

  return {
    cabecalho: {
      habilitada: habilitacao.habilitado,
      origem: origemDe(estado.mapeamento, condominio, caso.nome),
      travado,
      onMudarHabilitada: (ligado: boolean) => void gravar(ligado, habilitacao.papeis),
    },
    quem: {
      papeis: estado.papeis,
      escolhidos: habilitacao.papeis,
      travado,
      onMudar: (papeis: string[]) => void gravar(habilitacao.habilitado, papeis),
    },
    rodape: { podeVoltar: propria && editavel, gravando, onVoltar: () => void voltarAoPadrao() },
  };
}

type RodapeProps = { caso: Caso; podeVoltar: boolean; gravando: boolean; onVoltar: () => void };

function RodapeDoDrawer({ caso, podeVoltar, gravando, onVoltar }: RodapeProps): ReactNode {
  const { data: eu } = useEu();
  return (
    <Box sx={{ display: 'flex', gap: 1, mt: 'auto', pt: 2 }}>
      {eu && pode(eu, 'skills.ler') && <VerOFluxo nome={caso.nome} />}
      {podeVoltar && (
        <Button
          variant="contained"
          disableElevation
          fullWidth
          onClick={onVoltar}
          disabled={gravando}
          sx={{ bgcolor: 'background.neutralFill', color: 'text.primary' }}
        >
          Voltar ao Padrão
        </Button>
      )}
    </Box>
  );
}

function VerOFluxo({ nome }: { nome: string }): ReactNode {
  return (
    <Button
      variant="outlined"
      fullWidth
      component={Link}
      to={`/skills/${encodeURIComponent(nome)}?aba=fluxo`}
    >
      Ver o fluxo
    </Button>
  );
}
