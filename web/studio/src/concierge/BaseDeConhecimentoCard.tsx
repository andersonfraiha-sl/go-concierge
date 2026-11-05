import AddIcon from '@mui/icons-material/Add';
import DeleteOutlinedIcon from '@mui/icons-material/DeleteOutlined';
import DescriptionOutlinedIcon from '@mui/icons-material/DescriptionOutlined';
import { Box, Button, IconButton, Paper, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { useState } from 'react';
import { usePodeEditar } from '@/acesso/eu';
import type { Arquivo } from '@/catalogo/contratos';
import { useAgente } from '@/catalogo/contratos';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { EmptyState } from '@/shared/ui/EmptyState';
import { ErroComRetry } from '@/shared/ui/ErroComRetry';
import { NomeDialog } from '@/shared/ui/NomeDialog';
import {
  documentosDe,
  formatarTamanho,
  nomeParaExibir,
  tituloDoDocumento,
  useGestaoDeArquivos,
} from '@/skills/abas/arquivosDaSkill';
import { ESPACOS, ICONES, PESOS, RAIO, SHELL } from '@/theme/tokens';
import { ConciergeCard } from './ConciergeCard';

const extensaoDe = (caminho: string): string => {
  const ponto = caminho.lastIndexOf('.');
  return ponto === -1 ? '' : caminho.slice(ponto + 1).toUpperCase();
};

/**
 * Lista de documentos fechada (board): troca o `ConhecimentoAba` de skill (fora de escopo, em
 * `src/skills/abas`), que mistura linha fechada com editor expandido conforme a extensão.
 */
export function BaseDeConhecimentoCard({ assistente }: { assistente: string }): ReactNode {
  const agente = useAgente(assistente);
  const editavel = usePodeEditar('skills.editar_textos');
  const documentos = agente.data ? documentosDe(agente.data.agente) : [];
  const { criar, excluir, emExclusao } = useGestaoDeArquivos(assistente, 'documento', documentos);
  const [criando, setCriando] = useState(false);
  const [excluindo, setExcluindo] = useState<string | null>(null);

  const botaoNovo = editavel && (
    <Button
      size="small"
      startIcon={<AddIcon />}
      onClick={() => setCriando(true)}
      sx={{
        fontWeight: PESOS.semi,
        px: `${ESPACOS[1]}px`,
        py: 0,
        height: SHELL.alturaDoControle,
        lineHeight: 'normal',
        borderRadius: RAIO.chip,
      }}
    >
      Novo documento
    </Button>
  );

  return (
    <ConciergeCard
      titulo="Base de conhecimento comum"
      ajuda="Documentos que todas as skills consultam."
      acao={botaoNovo}
    >
      {agente.isPending && <Typography variant="caption">Carregando…</Typography>}
      {agente.isError && (
        <ErroComRetry
          mensagem="Não foi possível carregar os documentos."
          onTentarDeNovo={() => void agente.refetch()}
        />
      )}
      {agente.isSuccess && documentos.length === 0 && (
        <EmptyState frase="Esta skill ainda não tem documentos." />
      )}
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
        {documentos.map((documento) => (
          <LinhaDeDocumento
            key={documento.caminho}
            documento={documento}
            emExclusao={emExclusao === documento.caminho}
            onExcluir={editavel ? () => setExcluindo(documento.caminho) : undefined}
          />
        ))}
      </Box>
      {criando && (
        <NomeDialog
          titulo="Novo documento"
          tituloInicial=""
          confirmar="Criar"
          onEnviar={async ({ nome }) => {
            await criar(nome);
            setCriando(false);
          }}
          onCancelar={() => setCriando(false)}
        />
      )}
      <ConfirmDialog
        aberto={excluindo !== null}
        titulo="Excluir documento"
        texto={`Excluir "${excluindo === null ? '' : tituloDoDocumento(excluindo)}"? A skill deixa de consultá-lo.`}
        confirmar="Excluir"
        onCancelar={() => setExcluindo(null)}
        onConfirmar={() => {
          if (excluindo !== null) void excluir(excluindo);
          setExcluindo(null);
        }}
      />
    </ConciergeCard>
  );
}

type PropsDaLinha = {
  documento: Arquivo;
  emExclusao: boolean;
  onExcluir?: (() => void) | undefined;
};

function LinhaDeDocumento({ documento, emExclusao, onExcluir }: PropsDaLinha): ReactNode {
  return (
    <Paper
      variant="outlined"
      sx={{
        p: 2,
        display: 'flex',
        alignItems: 'center',
        gap: 1,
        borderColor: 'border.default',
        opacity: emExclusao ? 0.5 : 1,
      }}
    >
      <DescriptionOutlinedIcon sx={{ fontSize: ICONES.grande, color: 'text.tertiary' }} />
      <Typography variant="body1" sx={{ flex: 1, fontWeight: PESOS.semi }}>
        {nomeParaExibir(documento)}
      </Typography>
      <Typography variant="body2" color="text.secondary">
        {extensaoDe(documento.caminho)} · {formatarTamanho(documento.bytes)}
      </Typography>
      {onExcluir && (
        <IconButton
          size="small"
          aria-label={`Excluir ${nomeParaExibir(documento)}`}
          onClick={onExcluir}
          disabled={emExclusao}
          sx={{ borderRadius: RAIO.canto }}
        >
          <DeleteOutlinedIcon fontSize="small" />
        </IconButton>
      )}
    </Paper>
  );
}
