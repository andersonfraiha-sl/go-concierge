import {
  Box,
  Chip,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from '@mui/material';
import { type ReactNode, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router';
import { useRealceDoCopiloto } from '@/copiloto/CopilotoProvider';
import { contemBusca } from '@/shared/busca';
import { EmptyState } from '@/shared/ui/EmptyState';
import { PESOS } from '@/theme/tokens';
import {
  type Administradora,
  type Condominio,
  type Estado,
  faltamIds,
  PADRAO,
  useEscopo,
} from './contratos';
import { contarLigadas, linhasDeSkills } from './listaDeSkills';

type Situacao = 'base' | 'pronto' | 'faltam-ids' | 'sem-administradora';

/** Cores do badge de Situação: tons do desenho aprovado, vindos de tokens.ts. */
const BADGE_DA_SITUACAO: Record<Situacao, { rotulo: string; fundo: string; cor: string }> = {
  base: { rotulo: 'Base', fundo: 'background.tint', cor: 'primary.main' },
  pronto: {
    rotulo: 'Pronto',
    fundo: 'success.soft',
    cor: 'success.main',
  },
  'faltam-ids': {
    rotulo: 'Faltam ids',
    fundo: 'warning.soft',
    cor: 'warning.onSoft',
  },
  'sem-administradora': {
    rotulo: 'Sem administradora',
    fundo: 'warning.soft',
    cor: 'warning.onSoft',
  },
};

/** Sem ids trava a integração (mais grave); sem administradora é o próximo alerta; senão, pronto. */
function situacaoDoCondominio(condominio: Condominio): Situacao {
  if (faltamIds(condominio)) return 'faltam-ids';
  if (condominio.administradora_id === '') return 'sem-administradora';
  return 'pronto';
}

function SituacaoBadge({ situacao }: { situacao: Situacao }): ReactNode {
  const badge = BADGE_DA_SITUACAO[situacao];
  return (
    <Chip
      size="small"
      label={badge.rotulo}
      sx={{ bgcolor: badge.fundo, color: badge.cor, fontWeight: PESOS.medio }}
    />
  );
}

const nomeDaAdministradora = (estado: Estado, condominio: Condominio): string =>
  estado.administradoras.find((a) => a.id === condominio.administradora_id)?.nome ?? '';

function sem(atual: URLSearchParams, chave: string): URLSearchParams {
  const proximos = new URLSearchParams(atual);
  proximos.delete(chave);
  return proximos;
}

function useCondominiosVisiveis(estado: Estado, busca: string) {
  const [params, setParams] = useSearchParams();
  const idDaAdministradora = params.get('administradora');
  const visiveis = estado.mapeamento.condominios
    .filter((c) => idDaAdministradora === null || c.administradora_id === idDaAdministradora)
    .filter((c) => contemBusca([c.nome, nomeDaAdministradora(estado, c)], busca));

  return {
    visiveis,
    administradoraFiltro: estado.administradoras.find((a) => a.id === idDaAdministradora),
    limparFiltro: () => setParams((atual) => sem(atual, 'administradora'), { replace: true }),
  };
}

export function TabelaDeCondominios({ estado }: { estado: Estado }): ReactNode {
  const [busca, setBusca] = useState('');
  const { visiveis, administradoraFiltro, limparFiltro } = useCondominiosVisiveis(estado, busca);

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <FiltrosDaTabela
        busca={busca}
        onBuscar={setBusca}
        administradoraFiltro={administradoraFiltro}
        onLimpar={limparFiltro}
      />
      <GradeDeCondominios estado={estado} visiveis={visiveis} />
      <VaziosDaTabela total={estado.mapeamento.condominios.length} visiveis={visiveis.length} />
    </Box>
  );
}

type CorpoProps = { estado: Estado; visiveis: Condominio[]; mostraAdministradora: boolean };

function GradeDeCondominios({ estado, visiveis }: Omit<CorpoProps, 'mostraAdministradora'>) {
  const mostraAdministradora = useEscopo() === null;
  return (
    <Paper variant="outlined" sx={{ overflow: 'hidden' }}>
      <Table size="small">
        <CabecalhoDaTabela mostraAdministradora={mostraAdministradora} />
        <CorpoDaTabela
          estado={estado}
          visiveis={visiveis}
          mostraAdministradora={mostraAdministradora}
        />
      </Table>
    </Paper>
  );
}

type FiltrosProps = {
  busca: string;
  onBuscar: (busca: string) => void;
  administradoraFiltro: Administradora | undefined;
  onLimpar: () => void;
};

function FiltrosDaTabela({ busca, onBuscar, administradoraFiltro, onLimpar }: FiltrosProps) {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
      <TextField
        label="Buscar condomínio"
        size="small"
        value={busca}
        onChange={(e) => onBuscar(e.target.value)}
        sx={{ width: 300 }}
      />
      {administradoraFiltro && (
        <Chip label={`Administradora: ${administradoraFiltro.nome}`} onDelete={onLimpar} />
      )}
    </Box>
  );
}

function CabecalhoDaTabela({ mostraAdministradora }: { mostraAdministradora: boolean }) {
  return (
    <TableHead>
      <TableRow>
        <TableCell scope="col">Condomínio</TableCell>
        {mostraAdministradora && <TableCell scope="col">Administradora</TableCell>}
        <TableCell scope="col">Skills ativas</TableCell>
        <TableCell scope="col">Situação</TableCell>
      </TableRow>
    </TableHead>
  );
}

const LINHA_DO_PADRAO = {
  id: PADRAO,
  nome: 'Padrão (todos)',
  legenda: 'Vale para todo condomínio que não tem decisão própria.',
  administradora: '',
  situacao: 'base',
} as const;

function CorpoDaTabela({ estado, visiveis, mostraAdministradora }: CorpoProps): ReactNode {
  const skillsDe = contagemDeSkills(estado);

  return (
    <TableBody>
      <LinhaDeCondominio
        {...LINHA_DO_PADRAO}
        mostraAdministradora={mostraAdministradora}
        skills={skillsDe(PADRAO)}
      />
      {visiveis.map((condominio) => (
        <LinhaDeCondominio
          key={condominio.id}
          {...dadosDaLinha(estado, condominio)}
          mostraAdministradora={mostraAdministradora}
          skills={skillsDe(condominio.id)}
        />
      ))}
    </TableBody>
  );
}

function contagemDeSkills(estado: Estado): (id: string) => { ligadas: number; total: number } {
  const titulos = new Map(estado.casos.map((caso) => [caso.nome, caso.titulo]));
  return (id) => contarLigadas(linhasDeSkills(estado, id, titulos));
}

function dadosDaLinha(estado: Estado, condominio: Condominio) {
  const { id, nome } = condominio;
  const administradora = nomeDaAdministradora(estado, condominio);
  return { id, nome, administradora, situacao: situacaoDoCondominio(condominio) };
}

function VaziosDaTabela({ total, visiveis }: { total: number; visiveis: number }): ReactNode {
  if (total === 0) return <EmptyState frase="Nenhum condomínio ainda." />;
  if (visiveis === 0) return <EmptyState frase="Nenhum condomínio encontrado." />;
  return null;
}

type LinhaProps = {
  id: string;
  nome: string;
  administradora: string;
  legenda?: string;
  situacao: Situacao;
  mostraAdministradora: boolean;
  skills: { ligadas: number; total: number };
};

function LinhaDeCondominio(props: LinhaProps): ReactNode {
  const { id, skills, situacao } = props;
  const navegar = useNavigate();
  const destino = `/condominios/${encodeURIComponent(id)}`;
  const realce = useRealceDoCopiloto(`condominio:${id}`);

  return (
    <TableRow hover onClick={() => navegar(destino)} sx={[{ cursor: 'pointer' }, realce]}>
      <CelulaDoNome destino={destino} nome={props.nome} legenda={props.legenda} />
      {props.mostraAdministradora && <CelulaDaAdministradora nome={props.administradora} />}
      <TableCell>{`${skills.ligadas} de ${skills.total}`}</TableCell>
      <TableCell>
        <SituacaoBadge situacao={situacao} />
      </TableCell>
    </TableRow>
  );
}

type NomeProps = { destino: string; nome: string; legenda: string | undefined };

function CelulaDoNome({ destino, nome, legenda }: NomeProps): ReactNode {
  return (
    <TableCell>
      <Link to={destino} style={{ fontWeight: PESOS.semi }} onClick={(e) => e.stopPropagation()}>
        {nome}
      </Link>
      {legenda && <Legenda texto={legenda} />}
    </TableCell>
  );
}

function Legenda({ texto }: { texto: string }): ReactNode {
  return (
    <Typography variant="caption" color="text.secondary" component="div">
      {texto}
    </Typography>
  );
}

function CelulaDaAdministradora({ nome }: { nome: string }): ReactNode {
  return (
    <TableCell>
      <Typography variant="body2" color={nome === '' ? 'text.disabled' : undefined}>
        {nome === '' ? '—' : nome}
      </Typography>
    </TableCell>
  );
}
