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
import { tokens } from '@/theme/tokens';
import { type Condominio, type Estado, faltamIds, PADRAO, useEscopo } from './contratos';
import { contarLigadas, linhasDeSkills } from './listaDeSkills';

type Situacao = 'base' | 'pronto' | 'faltam-ids' | 'sem-administradora';

/** Cores do badge de Situação: tons do desenho aprovado, vindos de tokens.ts. */
const BADGE_DA_SITUACAO: Record<Situacao, { rotulo: string; fundo: string; cor: string }> = {
  base: { rotulo: 'Base', fundo: tokens.color.background.tint, cor: tokens.color.primary.main },
  pronto: {
    rotulo: 'Pronto',
    fundo: tokens.color.success.background,
    cor: tokens.color.success.main,
  },
  'faltam-ids': {
    rotulo: 'Faltam ids',
    fundo: tokens.color.warning.background,
    cor: tokens.color.warning.text,
  },
  'sem-administradora': {
    rotulo: 'Sem administradora',
    fundo: tokens.color.warning.background,
    cor: tokens.color.warning.text,
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
      sx={{ bgcolor: badge.fundo, color: badge.cor, fontWeight: 500 }}
    />
  );
}

export function TabelaDeCondominios({ estado }: { estado: Estado }): ReactNode {
  const [params, setParams] = useSearchParams();
  const escopo = useEscopo();
  const [busca, setBusca] = useState('');
  const mostraAdministradora = escopo === null;
  const idDaAdministradora = params.get('administradora');
  const administradoraFiltro = estado.administradoras.find((a) => a.id === idDaAdministradora);
  const titulos = new Map(estado.casos.map((caso) => [caso.nome, caso.titulo]));

  const donaDe = (condominio: Condominio): string =>
    estado.administradoras.find((a) => a.id === condominio.administradora_id)?.nome ?? '';

  const { condominios } = estado.mapeamento;
  const visiveis = condominios
    .filter((c) => idDaAdministradora === null || c.administradora_id === idDaAdministradora)
    .filter((c) => contemBusca([c.nome, donaDe(c)], busca));

  const limparFiltroDaAdministradora = (): void =>
    setParams(
      (atual) => {
        const proximos = new URLSearchParams(atual);
        proximos.delete('administradora');
        return proximos;
      },
      { replace: true },
    );

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
        <TextField
          label="Buscar condomínio"
          size="small"
          value={busca}
          onChange={(e) => setBusca(e.target.value)}
          sx={{ width: 300 }}
        />
        {administradoraFiltro && (
          <Chip
            label={`Administradora: ${administradoraFiltro.nome}`}
            onDelete={limparFiltroDaAdministradora}
          />
        )}
      </Box>

      <Paper variant="outlined" sx={{ overflow: 'hidden' }}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell scope="col">Condomínio</TableCell>
              {mostraAdministradora && <TableCell scope="col">Administradora</TableCell>}
              <TableCell scope="col">Skills ativas</TableCell>
              <TableCell scope="col">Situação</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            <LinhaDeCondominio
              id={PADRAO}
              nome="Padrão (todos)"
              legenda="Vale para todo condomínio que não tem decisão própria."
              administradora=""
              situacao="base"
              mostraAdministradora={mostraAdministradora}
              skills={contarLigadas(linhasDeSkills(estado, PADRAO, titulos))}
            />
            {visiveis.map((condominio) => (
              <LinhaDeCondominio
                key={condominio.id}
                id={condominio.id}
                nome={condominio.nome}
                administradora={donaDe(condominio)}
                situacao={situacaoDoCondominio(condominio)}
                mostraAdministradora={mostraAdministradora}
                skills={contarLigadas(linhasDeSkills(estado, condominio.id, titulos))}
              />
            ))}
          </TableBody>
        </Table>
      </Paper>

      {condominios.length === 0 && <EmptyState frase="Nenhum condomínio ainda." />}
      {condominios.length > 0 && visiveis.length === 0 && (
        <EmptyState frase="Nenhum condomínio encontrado." />
      )}
    </Box>
  );
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

function LinhaDeCondominio({
  id,
  nome,
  administradora,
  legenda,
  situacao,
  mostraAdministradora,
  skills,
}: LinhaProps): ReactNode {
  const navegar = useNavigate();
  const destino = `/condominios/${encodeURIComponent(id)}`;
  const realce = useRealceDoCopiloto(`condominio:${id}`);

  return (
    <TableRow hover onClick={() => navegar(destino)} sx={[{ cursor: 'pointer' }, realce]}>
      <TableCell>
        <Link to={destino} style={{ fontWeight: 600 }} onClick={(e) => e.stopPropagation()}>
          {nome}
        </Link>
        {legenda && (
          <Typography variant="caption" color="text.secondary" component="div">
            {legenda}
          </Typography>
        )}
      </TableCell>
      {mostraAdministradora && (
        <TableCell>
          <Typography variant="body2" color={administradora === '' ? 'text.disabled' : undefined}>
            {administradora === '' ? '—' : administradora}
          </Typography>
        </TableCell>
      )}
      <TableCell>{`${skills.ligadas} de ${skills.total}`}</TableCell>
      <TableCell>
        <SituacaoBadge situacao={situacao} />
      </TableCell>
    </TableRow>
  );
}
