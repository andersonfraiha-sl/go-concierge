import AddIcon from '@mui/icons-material/Add';
import DeleteOutlinedIcon from '@mui/icons-material/DeleteOutlined';
import { Box, Button, IconButton, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { useState } from 'react';
import { usePodeEditar } from '@/acesso/eu';
import type { Arquivo } from '@/catalogo/contratos';
import { useAgente } from '@/catalogo/contratos';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { EmptyState } from '@/shared/ui/EmptyState';
import { ErroComRetry } from '@/shared/ui/ErroComRetry';
import { NomeDialog } from '@/shared/ui/NomeDialog';
import { type regrasDe, useGestaoDeArquivos } from '@/skills/abas/arquivosDaSkill';
import { ESPACOS, PESOS, RAIO, SHELL } from '@/theme/tokens';
import { ConciergeCard } from './ConciergeCard';

export type ItemDaLista = {
  arquivo: Arquivo;
  emExclusao: boolean;
  /** Ausente para quem não pode editar. */
  onExcluir: (() => void) | undefined;
};

type PropsDaLista = {
  assistente: string;
  tipo: 'regra' | 'documento';
  titulo: string;
  ajuda: string;
  rotuloDoNovo: string;
  textos: { erro: string; vazio: string; excluir: (caminho: string) => string };
  /** regrasDe ou documentosDe. */
  arquivosDe: typeof regrasDe;
  item: (props: ItemDaLista) => ReactNode;
};

/** Cartão do Concierge com arquivos fechados (regras ou documentos): criar, listar e excluir. */
export function ListaDeArquivosCard(props: PropsDaLista): ReactNode {
  const agente = useAgente(props.assistente);
  const editavel = usePodeEditar('skills.editar_textos');
  const arquivos = agente.data ? props.arquivosDe(agente.data.agente) : [];
  const gestao = useGestaoDeArquivos(props.assistente, props.tipo, arquivos);
  const [criando, setCriando] = useState(false);
  const [excluindo, setExcluindo] = useState<string | null>(null);

  const novo = editavel && (
    <BotaoNovo rotulo={props.rotuloDoNovo} onClick={() => setCriando(true)} />
  );
  return (
    <ConciergeCard titulo={props.titulo} ajuda={props.ajuda} acao={novo}>
      {agente.isPending && <Typography variant="caption">Carregando…</Typography>}
      {agente.isError && (
        <ErroComRetry mensagem={props.textos.erro} onTentarDeNovo={() => void agente.refetch()} />
      )}
      {agente.isSuccess && arquivos.length === 0 && <EmptyState frase={props.textos.vazio} />}
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
        {arquivos.map((arquivo) => (
          <Box key={arquivo.caminho}>
            {props.item({
              arquivo,
              emExclusao: gestao.emExclusao === arquivo.caminho,
              onExcluir: editavel ? () => setExcluindo(arquivo.caminho) : undefined,
            })}
          </Box>
        ))}
      </Box>
      {criando && (
        <NomeDialog
          titulo={props.rotuloDoNovo}
          tituloInicial=""
          confirmar="Criar"
          onEnviar={async ({ nome }) => {
            await gestao.criar(nome);
            setCriando(false);
          }}
          onCancelar={() => setCriando(false)}
        />
      )}
      <ConfirmDialog
        aberto={excluindo !== null}
        titulo={`Excluir ${props.tipo}`}
        texto={excluindo === null ? '' : props.textos.excluir(excluindo)}
        confirmar="Excluir"
        onCancelar={() => setExcluindo(null)}
        onConfirmar={() => {
          if (excluindo !== null) void gestao.excluir(excluindo);
          setExcluindo(null);
        }}
      />
    </ConciergeCard>
  );
}

function BotaoNovo({ rotulo, onClick }: { rotulo: string; onClick: () => void }): ReactNode {
  return (
    <Button
      size="small"
      startIcon={<AddIcon />}
      onClick={onClick}
      sx={{
        fontWeight: PESOS.semi,
        px: `${ESPACOS[1]}px`,
        py: 0,
        height: SHELL.alturaDoControle,
        lineHeight: 'normal',
        borderRadius: RAIO.chip,
      }}
    >
      {rotulo}
    </Button>
  );
}

type PropsDoExcluir = { nome: string; emExclusao: boolean; onExcluir: () => void };

export function BotaoExcluir({ nome, emExclusao, onExcluir }: PropsDoExcluir): ReactNode {
  return (
    <IconButton
      size="small"
      aria-label={`Excluir ${nome}`}
      onClick={onExcluir}
      disabled={emExclusao}
      sx={{ borderRadius: RAIO.canto }}
    >
      <DeleteOutlinedIcon fontSize="small" />
    </IconButton>
  );
}
