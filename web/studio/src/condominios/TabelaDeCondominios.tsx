import ChevronRightOutlined from '@mui/icons-material/ChevronRightOutlined';
import SearchOutlined from '@mui/icons-material/SearchOutlined';
import {
  Box,
  Chip,
  InputAdornment,
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
import { CONDOMINIOS, PESOS, RAIOS, TIPOS } from '@/theme/tokens';
import {
  type Administradora,
  type Condominio,
  type Estado,
  faltamIds,
  PADRAO,
  useEscopo,
} from './contratos';
import { contarLigadas, linhasDeSkills } from './listaDeSkills';
import { OpcoesDeAdministradora } from './SeletorDeAdministradora';

type Situacao = 'base' | 'pronto' | 'faltam-ids' | 'sem-administradora';

/** Cores do badge de Situação: tons do desenho aprovado, vindos de tokens.ts. */
const BADGE_DA_SITUACAO: Record<Situacao, { rotulo: string; fundo: string; cor: string }> = {
  base: { rotulo: 'Base', fundo: 'background.tint', cor: 'primary.main' },
  pronto: {
    rotulo: 'Pronto',
    fundo: 'success.soft',
    // onSoft (não main): main dava 3.3:1 sobre o fundo suave, abaixo do mínimo AA de 4.5:1
    // (achado pelo e2e de acessibilidade). onSoft já é o tom pensado para texto sobre soft.
    cor: 'success.onSoft',
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

const ESTILO_DO_BADGE = {
  height: 'auto',
  borderRadius: `${RAIOS.chip}px`,
  px: 1,
  py: 0.375,
  '& .MuiChip-label': { px: 0, fontSize: `${TIPOS.overline}px`, lineHeight: 1 },
} as const;

function SituacaoBadge({ situacao }: { situacao: Situacao }): ReactNode {
  const badge = BADGE_DA_SITUACAO[situacao];
  return (
    <Chip
      size="small"
      label={badge.rotulo}
      sx={[ESTILO_DO_BADGE, { bgcolor: badge.fundo, color: badge.cor, fontWeight: PESOS.medio }]}
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
  const idDaAdministradora = params.get('administradora') ?? '';
  const visiveis = estado.mapeamento.condominios
    .filter((c) => idDaAdministradora === '' || c.administradora_id === idDaAdministradora)
    .filter((c) => contemBusca([c.nome, nomeDaAdministradora(estado, c)], busca));

  return {
    visiveis,
    idDaAdministradora,
    escolherAdministradora: (id: string) =>
      setParams((atual) => (id === '' ? sem(atual, 'administradora') : com(atual, id)), {
        replace: true,
      }),
  };
}

function com(atual: URLSearchParams, idDaAdministradora: string): URLSearchParams {
  const proximos = new URLSearchParams(atual);
  proximos.set('administradora', idDaAdministradora);
  return proximos;
}

/** "6 condomínios · 2 precisam de atenção" (faltam ids ou administradora), alinhado à direita. */
function resumoDosCondominios(condominios: readonly Condominio[]): string {
  const comAtencao = condominios.filter((c) => faltamIds(c) || c.administradora_id === '').length;
  const base = condominios.length === 1 ? '1 condomínio' : `${condominios.length} condomínios`;
  if (comAtencao === 0) return base;
  const atencao = comAtencao === 1 ? '1 precisa de atenção' : `${comAtencao} precisam de atenção`;
  return `${base} · ${atencao}`;
}

export function TabelaDeCondominios({ estado }: { estado: Estado }): ReactNode {
  const [busca, setBusca] = useState('');
  const { visiveis, idDaAdministradora, escolherAdministradora } = useCondominiosVisiveis(
    estado,
    busca,
  );

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <FiltrosDaTabela
        busca={busca}
        onBuscar={setBusca}
        administradoras={estado.administradoras}
        idDaAdministradora={idDaAdministradora}
        onEscolherAdministradora={escolherAdministradora}
        resumo={resumoDosCondominios(estado.mapeamento.condominios)}
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
    <Paper variant="outlined" sx={{ overflow: 'hidden', borderRadius: `${RAIOS.cartao}px` }}>
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
  administradoras: Administradora[];
  idDaAdministradora: string;
  onEscolherAdministradora: (id: string) => void;
  resumo: string;
};

const ADORNO_DA_BUSCA = {
  startAdornment: (
    <InputAdornment position="start">
      <SearchOutlined fontSize="small" sx={{ color: 'text.faint' }} />
    </InputAdornment>
  ),
};

function FiltrosDaTabela({
  busca,
  onBuscar,
  administradoras,
  idDaAdministradora,
  onEscolherAdministradora,
  resumo,
}: FiltrosProps) {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
      <TextField
        placeholder="Buscar condomínio"
        size="small"
        value={busca}
        onChange={(e) => onBuscar(e.target.value)}
        slotProps={{ input: ADORNO_DA_BUSCA, htmlInput: { 'aria-label': 'Buscar condomínio' } }}
        sx={{ width: CONDOMINIOS.larguraDaBusca }}
      />
      <TextField
        select
        size="small"
        value={idDaAdministradora}
        onChange={(e) => onEscolherAdministradora(e.target.value)}
        sx={{ width: CONDOMINIOS.larguraDoFiltro }}
        slotProps={{ select: { native: true, 'aria-label': 'Administradora' } }}
      >
        <option value="">Todas as administradoras</option>
        <OpcoesDeAdministradora administradoras={administradoras} />
      </TextField>
      <Box sx={{ flex: 1 }} />
      <Typography variant="body2" color="textTertiary">
        {resumo}
      </Typography>
    </Box>
  );
}

const ESTILO_DO_CABECALHO = {
  bgcolor: 'background.subtle',
  color: 'text.tertiary',
  typography: 'overline',
  fontWeight: PESOS.semi,
  textTransform: 'uppercase',
  py: 1.25,
  px: 3,
} as const;

function CabecalhoDaTabela({ mostraAdministradora }: { mostraAdministradora: boolean }) {
  return (
    <TableHead>
      <TableRow>
        <TableCell scope="col" sx={ESTILO_DO_CABECALHO}>
          Condomínio
        </TableCell>
        {mostraAdministradora && (
          <TableCell scope="col" sx={ESTILO_DO_CABECALHO}>
            Administradora
          </TableCell>
        )}
        <TableCell scope="col" sx={ESTILO_DO_CABECALHO}>
          Skills ativas
        </TableCell>
        <TableCell scope="col" sx={ESTILO_DO_CABECALHO}>
          Situação
        </TableCell>
        <TableCell scope="col" sx={ESTILO_DO_CABECALHO} aria-hidden />
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

const ESTILO_DA_LINHA_DO_PADRAO = {
  borderLeft: 4,
  borderColor: 'primary.main',
  bgcolor: CONDOMINIOS.fundoDaLinhaPadrao,
} as const;

function LinhaDeCondominio(props: LinhaProps): ReactNode {
  const { id, skills, situacao } = props;
  const ehPadrao = id === PADRAO;
  const navegar = useNavigate();
  const destino = `/condominios/${encodeURIComponent(id)}`;
  const realce = useRealceDoCopiloto(`condominio:${id}`);

  return (
    <TableRow
      hover
      onClick={() => navegar(destino)}
      sx={[{ cursor: 'pointer' }, ehPadrao && ESTILO_DA_LINHA_DO_PADRAO, realce]}
    >
      <CelulaDoNome
        destino={destino}
        nome={props.nome}
        legenda={props.legenda}
        ehPadrao={ehPadrao}
      />
      {props.mostraAdministradora && <CelulaDaAdministradora nome={props.administradora} />}
      <TableCell>{`${skills.ligadas} de ${skills.total}`}</TableCell>
      <TableCell>
        <SituacaoBadge situacao={situacao} />
      </TableCell>
      <TableCell>
        <ChevronRightOutlined fontSize="small" sx={{ color: 'text.muted', display: 'block' }} />
      </TableCell>
    </TableRow>
  );
}

type NomeProps = {
  destino: string;
  nome: string;
  legenda: string | undefined;
  ehPadrao: boolean;
};

function CelulaDoNome({ destino, nome, legenda, ehPadrao }: NomeProps): ReactNode {
  return (
    <TableCell>
      <Typography
        component={Link}
        to={destino}
        onClick={(e) => e.stopPropagation()}
        sx={{
          fontWeight: PESOS.semi,
          textDecoration: 'none',
          color: ehPadrao ? 'secondary.main' : 'text.primary',
        }}
      >
        {nome}
      </Typography>
      {legenda && <Legenda texto={legenda} />}
    </TableCell>
  );
}

function Legenda({ texto }: { texto: string }): ReactNode {
  return (
    <Typography variant="caption" color="textSecondary" component="div">
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
