import AddIcon from '@mui/icons-material/Add';
import { Box, Button, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { usePodeEditar } from '@/acesso/eu';
import { useAgente } from '@/catalogo/contratos';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { EmptyState } from '@/shared/ui/EmptyState';
import { ErroComRetry } from '@/shared/ui/ErroComRetry';
import { NomeDialog } from '@/shared/ui/NomeDialog';
import { SectionCard } from '@/shared/ui/SectionCard';
import { regrasDe, tituloDaRegra, useGestaoDeArquivos } from './arquivosDaSkill';
import { DocumentoEditavel } from './DocumentoEditavel';

type Props = { nome: string; caminho: string };

export function RegrasAba({ caminho }: Props): ReactNode {
  const agente = useAgente(caminho);
  const editavel = usePodeEditar('skills.editar_textos');
  const regras = agente.data ? regrasDe(agente.data.agente) : [];
  const { criar, excluir, emExclusao } = useGestaoDeArquivos(caminho, 'regra', regras);
  const [criando, setCriando] = useState(false);
  const [excluindo, setExcluindo] = useState<string | null>(null);

  const botaoNova = editavel && (
    <Button size="small" startIcon={<AddIcon />} onClick={() => setCriando(true)}>
      Nova regra
    </Button>
  );

  return (
    <SectionCard
      titulo="Regras"
      ajuda="Orientações que a skill sempre segue ao responder."
      acao={botaoNova}
    >
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
          <DocumentoEditavel
            key={regra.caminho}
            agente={caminho}
            caminho={regra.caminho}
            titulo={tituloDaRegra(regra.caminho)}
            editavel={editavel}
            onExcluir={() => setExcluindo(regra.caminho)}
            descartar={emExclusao === regra.caminho}
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
    </SectionCard>
  );
}
