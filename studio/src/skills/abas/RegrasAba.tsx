import AddIcon from '@mui/icons-material/Add';
import { Alert, Box, Button, Typography } from '@mui/material';
import { useState, type ReactNode } from 'react';
import { useAgente } from '@/catalogo/contratos';
import { usePodeEditar } from '@/acesso/eu';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { EmptyState } from '@/shared/ui/EmptyState';
import { SectionCard } from '@/shared/ui/SectionCard';
import { NovaSkillDialog } from '../NovaSkillDialog';
import { regrasDe, tituloDaRegra, useGestaoDeArquivos } from './arquivosDaSkill';
import { DocumentoEditavel } from './DocumentoEditavel';

type Props = { nome: string; caminho: string };

export function RegrasAba({ caminho }: Props): ReactNode {
  const agente = useAgente(caminho);
  const editavel = usePodeEditar('skills.editar_textos');
  const regras = agente.data ? regrasDe(agente.data.agente) : [];
  const { criar, excluir } = useGestaoDeArquivos(caminho, 'regra', regras);
  const [criando, setCriando] = useState(false);
  const [excluindo, setExcluindo] = useState<string | null>(null);

  const botaoNova = editavel && (
    <Button size="small" startIcon={<AddIcon />} onClick={() => setCriando(true)}>
      Nova regra
    </Button>
  );

  return (
    <SectionCard titulo="Regras" ajuda="Orientações que a skill sempre segue ao responder." acao={botaoNova}>
      {agente.isPending && <Typography variant="caption">Carregando…</Typography>}
      {agente.isError && (
        <Alert
          severity="error"
          action={
            <Button color="inherit" onClick={() => void agente.refetch()}>
              Tentar de novo
            </Button>
          }
        >
          Não foi possível carregar as regras.
        </Alert>
      )}
      {agente.isSuccess && regras.length === 0 && <EmptyState frase="Esta skill ainda não tem regras." />}
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
        {regras.map((regra) => (
          <DocumentoEditavel
            key={regra.caminho}
            agente={caminho}
            caminho={regra.caminho}
            titulo={tituloDaRegra(regra.caminho)}
            editavel={editavel}
            onExcluir={() => setExcluindo(regra.caminho)}
          />
        ))}
      </Box>
      {criando && (
        <NovaSkillDialog
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
