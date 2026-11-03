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
import { contemBusca } from '@/shared/busca';
import { EmptyState } from '@/shared/ui/EmptyState';
import { type Condominio, type Estado, faltamIds, PADRAO, useEscopo } from './contratos';
import { contarLigadas, linhasDeSkills } from './listaDeSkills';

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
              <TableCell scope="col">Skills ligadas</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            <LinhaDeCondominio
              id={PADRAO}
              nome="Padrão (todos)"
              legenda="Vale para todo condomínio que não tem decisão própria."
              administradora=""
              chips={[]}
              mostraAdministradora={mostraAdministradora}
              skills={contarLigadas(linhasDeSkills(estado, PADRAO, titulos))}
            />
            {visiveis.map((condominio) => (
              <LinhaDeCondominio
                key={condominio.id}
                id={condominio.id}
                nome={condominio.nome}
                administradora={donaDe(condominio)}
                chips={chipsDoCondominio(condominio)}
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

function chipsDoCondominio(condominio: Condominio): string[] {
  const chips: string[] = [];
  if (condominio.administradora_id === '') chips.push('Sem administradora');
  if (faltamIds(condominio)) chips.push('Faltam ids');
  return chips;
}

type LinhaProps = {
  id: string;
  nome: string;
  administradora: string;
  legenda?: string;
  chips: readonly string[];
  mostraAdministradora: boolean;
  skills: { ligadas: number; total: number };
};

function LinhaDeCondominio({
  id,
  nome,
  administradora,
  legenda,
  chips,
  mostraAdministradora,
  skills,
}: LinhaProps): ReactNode {
  const navegar = useNavigate();
  const destino = `/condominios/${encodeURIComponent(id)}`;

  return (
    <TableRow hover onClick={() => navegar(destino)} sx={{ cursor: 'pointer' }}>
      <TableCell>
        <Link to={destino} style={{ fontWeight: 600 }} onClick={(e) => e.stopPropagation()}>
          {nome}
        </Link>
        {legenda && (
          <Typography variant="caption" color="text.secondary" component="div">
            {legenda}
          </Typography>
        )}
        {chips.length > 0 && (
          <Box sx={{ display: 'flex', gap: 0.5, mt: 0.5 }}>
            {chips.map((chip) => (
              <Chip key={chip} label={chip} size="small" />
            ))}
          </Box>
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
    </TableRow>
  );
}
