import AddIcon from '@mui/icons-material/Add';
import { Alert, Box, Button, Chip, Paper, Typography } from '@mui/material';
import { useState, type ReactNode } from 'react';
import { useAgente, type Arquivo } from '@/catalogo/contratos';
import { usePodeEditar } from '@/acesso/eu';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { EmptyState } from '@/shared/ui/EmptyState';
import { SectionCard } from '@/shared/ui/SectionCard';
import { NovaSkillDialog } from '../NovaSkillDialog';
import {
  documentosDe,
  documentosHerdadosDe,
  ehTexto,
  formatarTamanho,
  nomeParaExibir,
  tituloDoDocumento,
  useGestaoDeArquivos,
} from './arquivosDaSkill';
import { DocumentoEditavel } from './DocumentoEditavel';

type Props = { nome: string; caminho: string };

export function ConhecimentoAba({ caminho }: Props): ReactNode {
  const agente = useAgente(caminho);
  const editavel = usePodeEditar('skills.editar_textos');
  const proprios = agente.data ? documentosDe(agente.data.agente) : [];
  const herdados = agente.data ? documentosHerdadosDe(agente.data.agente) : [];
  const { criar, excluir } = useGestaoDeArquivos(caminho, 'documento', proprios);
  const [criando, setCriando] = useState(false);
  const [excluindo, setExcluindo] = useState<string | null>(null);

  const botaoNovo = editavel && (
    <Button size="small" startIcon={<AddIcon />} onClick={() => setCriando(true)}>
      Novo documento
    </Button>
  );

  return (
    <SectionCard titulo="Base de conhecimento" ajuda="Documentos que a skill consulta para responder." acao={botaoNovo}>
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
          Não foi possível carregar os documentos.
        </Alert>
      )}
      {agente.isSuccess && proprios.length + herdados.length === 0 && (
        <EmptyState frase="Esta skill ainda não tem documentos." />
      )}
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
        {proprios.map((documento) =>
          ehTexto(documento) ? (
            <DocumentoEditavel
              key={documento.caminho}
              agente={caminho}
              caminho={documento.caminho}
              titulo={tituloDoDocumento(documento.caminho)}
              editavel={editavel}
              onExcluir={() => setExcluindo(documento.caminho)}
            />
          ) : (
            <LinhaDeArquivo key={documento.caminho} arquivo={documento} />
          ),
        )}
        {herdados.map((documento) => (
          <LinhaDeArquivo key={documento.caminho} arquivo={documento} selo="Vem do Assistente" />
        ))}
      </Box>
      {criando && (
        <NovaSkillDialog
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
    </SectionCard>
  );
}

function LinhaDeArquivo({ arquivo, selo }: { arquivo: Arquivo; selo?: string }): ReactNode {
  return (
    <Paper variant="outlined" sx={{ p: 1.5, display: 'flex', alignItems: 'center', gap: 1 }}>
      <Typography variant="subtitle2" sx={{ flex: 1 }}>
        {nomeParaExibir(arquivo)}
      </Typography>
      {selo && <Chip size="small" variant="outlined" label={selo} />}
      <Typography variant="caption" color="text.secondary">
        {formatarTamanho(arquivo.bytes)}
      </Typography>
    </Paper>
  );
}
