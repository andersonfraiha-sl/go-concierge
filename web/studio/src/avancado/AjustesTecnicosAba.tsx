import {
  Box,
  Chip,
  MenuItem,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableRow,
  TextField,
  Typography,
} from '@mui/material';
import { type ReactNode, useState } from 'react';
import type { Ferramenta } from '@/catalogo/contratos';
import { ComEstado } from '@/condominios/ComEstado';
import { type Estado, PADRAO, salvarPayload, useEstado } from '@/condominios/contratos';
import { useExecutar } from '@/condominios/useExecutar';
import { tituloDe } from '@/ferramentas/contratos';
import { PayloadDoCondominioDrawer } from './PayloadDoCondominioDrawer';
import { type ParDoPayload, paresEmPayload } from './payloadLivre';

type Origem = 'próprio' | 'herdado' | null;

const temPayload = (estado: Estado, condominio: string, recurso: string): boolean =>
  Object.keys(estado.mapeamento.payloads[condominio]?.[recurso] ?? {}).length > 0;

const origemDe = (estado: Estado, condominio: string, recurso: string): Origem => {
  if (temPayload(estado, condominio, recurso)) return 'próprio';
  const herda = condominio !== PADRAO && temPayload(estado, PADRAO, recurso);
  return herda ? 'herdado' : null;
};

export function AjustesTecnicosAba({ editar }: { editar: boolean }): ReactNode {
  const consulta = useEstado();
  return (
    <ComEstado consulta={consulta}>
      {(estado) => <Conteudo estado={estado} editar={editar} />}
    </ComEstado>
  );
}

function Conteudo({ estado, editar }: { estado: Estado; editar: boolean }): ReactNode {
  const { executar, gravando } = useExecutar();
  const [condominio, setCondominio] = useState(PADRAO);
  const [aberto, setAberto] = useState<Ferramenta | null>(null);
  const opcoes = [
    { id: PADRAO, nome: 'Padrão (todos os condomínios)' },
    ...estado.mapeamento.condominios,
  ];
  const nomeAtual = opcoes.find((o) => o.id === condominio)?.nome ?? condominio;

  const gravar = async (recurso: Ferramenta, pares: readonly ParDoPayload[], apagar: boolean) => {
    const payload = apagar ? {} : paresEmPayload(pares);
    const ok = await executar(
      (escopo) => salvarPayload({ condominio, recurso: recurso.name, payload, apagar }, escopo),
      apagar ? 'Ajustes limpos.' : 'Ajustes salvos.',
    );
    if (ok) setAberto(null);
  };

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <TextField
        select
        size="small"
        label="Condomínio"
        value={condominio}
        onChange={(e) => setCondominio(e.target.value)}
        sx={{ maxWidth: 480 }}
      >
        {opcoes.map((o) => (
          <MenuItem key={o.id} value={o.id}>
            {o.nome}
          </MenuItem>
        ))}
      </TextField>
      {estado.recursos.length === 0 && (
        <Typography color="text.secondary">Ainda não há ferramentas cadastradas.</Typography>
      )}
      <Paper variant="outlined">
        <Table size="small" aria-label="Ferramentas">
          <TableBody>
            {estado.recursos.map((recurso) => {
              const origem = origemDe(estado, condominio, recurso.name);
              return (
                <TableRow key={recurso.name} hover>
                  <TableCell
                    component="th"
                    scope="row"
                    role="button"
                    tabIndex={0}
                    onClick={() => setAberto(recurso)}
                    onKeyDown={(e) => e.key === 'Enter' && setAberto(recurso)}
                    sx={{ cursor: 'pointer' }}
                  >
                    {tituloDe(recurso)}
                    {origem && <Chip size="small" label={origem} sx={{ ml: 1 }} />}
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </Paper>
      {aberto && (
        <PayloadDoCondominioDrawer
          key={`${condominio}/${aberto.name}`}
          recurso={aberto}
          nomeDoCondominio={nomeAtual}
          payload={estado.mapeamento.payloads[condominio]?.[aberto.name] ?? {}}
          podeEditar={editar}
          gravando={gravando}
          onSalvar={(pares) => void gravar(aberto, pares, false)}
          onLimpar={() => void gravar(aberto, [], true)}
          onFechar={() => setAberto(null)}
        />
      )}
    </Box>
  );
}
