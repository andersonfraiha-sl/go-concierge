import {
  Box,
  Button,
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
import {
  type Escopo,
  type Estado,
  PADRAO,
  salvarPayload,
  useEscopo,
  useEstado,
} from '@/condominios/contratos';
import { useExecutar } from '@/condominios/useExecutar';
import { tituloDe } from '@/ferramentas/contratos';
import { PESOS } from '@/theme/tokens';
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

type Opcao = { id: string; nome: string; escopo: Escopo; condominio: string };

function opcoesDeConteudo(estado: Estado, escopo: Escopo): Opcao[] {
  return [
    ...(escopo === null
      ? [{ id: 'global', nome: 'Conteúdo global', escopo: null, condominio: PADRAO }]
      : []),
    ...estado.administradoras.map((company) => ({
      id: JSON.stringify([company.id, null]),
      nome: `${company.nome} · Empresa`,
      escopo: company.id,
      condominio: PADRAO,
    })),
    ...estado.mapeamento.condominios.map((condo) => ({
      id: JSON.stringify([condo.administradora_id, condo.id]),
      nome: `${condo.nome} · ${condo.administradora_id}`,
      escopo: condo.administradora_id,
      condominio: condo.id,
    })),
  ];
}

function Conteudo({ estado, editar }: { estado: Estado; editar: boolean }): ReactNode {
  const escopo = useEscopo() ?? null;
  const opcoes = opcoesDeConteudo(estado, escopo);
  const [selecionado, setSelecionado] = useState<string>();
  const alvo = opcoes.find((opcao) => opcao.id === selecionado) ?? opcoes[0];
  const [aberto, setAberto] = useState<Ferramenta | null>(null);
  const alvoDaConsulta = alvo ? { escopo: alvo.escopo, condoID: alvo.condominio } : {};
  const consultaDoAlvo = useEstado({ enabled: alvo !== undefined, ...alvoDaConsulta });
  const { executar, gravando } = useExecutar(alvoDaConsulta);
  if (!alvo) return null;
  const { condominio } = alvo;
  const gravar = async (recurso: Ferramenta, pares: readonly ParDoPayload[], apagar: boolean) => {
    const payload = apagar ? {} : paresEmPayload(pares);
    const ok = await executar(
      (company) => salvarPayload({ condominio, recurso: recurso.name, payload, apagar }, company),
      apagar ? 'Ajustes limpos.' : 'Ajustes salvos.',
    );
    if (ok) setAberto(null);
  };

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <SeletorDeCondominio
        opcoes={opcoes}
        valor={alvo.id}
        onMudar={(id) => {
          setAberto(null);
          setSelecionado(id);
        }}
      />
      <ComEstado consulta={consultaDoAlvo}>
        {(configurado) => (
          <>
            {estado.recursos.length === 0 && (
              <Typography color="textSecondary">Ainda não há ferramentas cadastradas.</Typography>
            )}
            <TabelaDeFerramentas
              recursos={estado.recursos}
              origemDe={(recurso) => origemDe(configurado, condominio, recurso.name)}
              onAbrir={setAberto}
            />
            {aberto && (
              <PayloadDoCondominioDrawer
                key={`${alvo.id}/${aberto.name}`}
                recurso={aberto}
                nomeDoCondominio={alvo.nome}
                payload={configurado.mapeamento.payloads[condominio]?.[aberto.name] ?? {}}
                podeEditar={editar}
                gravando={gravando}
                onSalvar={(pares) => void gravar(aberto, pares, false)}
                onLimpar={() => void gravar(aberto, [], true)}
                onFechar={() => setAberto(null)}
              />
            )}
          </>
        )}
      </ComEstado>
    </Box>
  );
}

type SeletorProps = { opcoes: Opcao[]; valor: string; onMudar: (id: string) => void };

function SeletorDeCondominio({ opcoes, valor, onMudar }: SeletorProps): ReactNode {
  return (
    <TextField
      select
      size="small"
      label="Alvo do conteúdo"
      value={valor}
      onChange={(evento) => onMudar(evento.target.value)}
      sx={{ maxWidth: 480 }}
    >
      {opcoes.map((opcao) => (
        <MenuItem key={opcao.id} value={opcao.id}>
          {opcao.nome}
        </MenuItem>
      ))}
    </TextField>
  );
}

type TabelaProps = {
  recursos: readonly Ferramenta[];
  origemDe: (recurso: Ferramenta) => Origem;
  onAbrir: (recurso: Ferramenta) => void;
};

function TabelaDeFerramentas({ recursos, origemDe, onAbrir }: TabelaProps): ReactNode {
  return (
    <Paper variant="outlined">
      <Table size="small" aria-label="Ferramentas">
        <TableBody>
          {recursos.map((recurso) => (
            <LinhaDeFerramenta
              key={recurso.name}
              recurso={recurso}
              origem={origemDe(recurso)}
              onAbrir={onAbrir}
            />
          ))}
        </TableBody>
      </Table>
    </Paper>
  );
}

type LinhaProps = { recurso: Ferramenta; origem: Origem; onAbrir: (recurso: Ferramenta) => void };

function LinhaDeFerramenta({ recurso, origem, onAbrir }: LinhaProps): ReactNode {
  return (
    <TableRow hover>
      <TableCell component="th" scope="row">
        <Button
          onClick={() => onAbrir(recurso)}
          sx={{ textTransform: 'none', color: 'text.primary', fontWeight: PESOS.regular }}
        >
          {tituloDe(recurso)}
          {origem && <Chip size="small" label={origem} sx={{ ml: 1 }} />}
        </Button>
      </TableCell>
    </TableRow>
  );
}
