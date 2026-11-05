import AddIcon from '@mui/icons-material/Add';
import DeleteOutlinedIcon from '@mui/icons-material/DeleteOutlined';
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
import { regrasDe, tituloDaRegra, useGestaoDeArquivos } from '@/skills/abas/arquivosDaSkill';
import { ESPACOS, PESOS, RAIO, SHELL } from '@/theme/tokens';
import { ConciergeCard } from './ConciergeCard';

/**
 * Lista compacta de regras (board): troca o editor de texto livre do `RegrasAba` de skill (fora
 * de escopo aqui, em `src/skills/abas`) por cartões fechados com só título + resumo + excluir.
 */
export function RegrasGeraisCard({ assistente }: { assistente: string }): ReactNode {
  const agente = useAgente(assistente);
  const editavel = usePodeEditar('skills.editar_textos');
  const regras = agente.data ? regrasDe(agente.data.agente) : [];
  const { criar, excluir, emExclusao } = useGestaoDeArquivos(assistente, 'regra', regras);
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
      Nova regra
    </Button>
  );

  return (
    <ConciergeCard titulo="Regras gerais" ajuda="Valem para todas as skills." acao={botaoNovo}>
      {agente.isPending && <Typography variant="caption">Carregando…</Typography>}
      {agente.isError && (
        <ErroComRetry
          mensagem="Não foi possível carregar as regras."
          onTentarDeNovo={() => void agente.refetch()}
        />
      )}
      {agente.isSuccess && regras.length === 0 && (
        <EmptyState frase="Esta skill ainda não tem regras." />
      )}
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
        {regras.map((regra) => (
          <CartaoDeRegra
            key={regra.caminho}
            regra={regra}
            editavel={editavel}
            emExclusao={emExclusao === regra.caminho}
            onExcluir={() => setExcluindo(regra.caminho)}
          />
        ))}
      </Box>
      {criando && (
        <NomeDialog
          titulo="Nova regra"
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
        titulo="Excluir regra"
        texto={`Excluir a regra "${excluindo === null ? '' : tituloDaRegra(excluindo)}"? A skill deixa de segui-la.`}
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

type PropsDoCartao = {
  regra: Arquivo;
  editavel: boolean;
  emExclusao: boolean;
  onExcluir: () => void;
};

function CartaoDeRegra({ regra, editavel, emExclusao, onExcluir }: PropsDoCartao): ReactNode {
  const titulo = tituloDaRegra(regra.caminho);
  return (
    <Paper
      variant="outlined"
      sx={{
        p: 2,
        display: 'flex',
        alignItems: 'flex-start',
        gap: 1,
        borderColor: 'border.default',
        opacity: emExclusao ? 0.5 : 1,
      }}
    >
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography variant="body1" sx={{ fontWeight: PESOS.semi }}>
          {titulo}
        </Typography>
        {regra.resumo && (
          <Typography variant="body2" color="textSecondary">
            {regra.resumo}
          </Typography>
        )}
      </Box>
      {editavel && (
        <IconButton
          size="small"
          aria-label={`Excluir ${titulo}`}
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
