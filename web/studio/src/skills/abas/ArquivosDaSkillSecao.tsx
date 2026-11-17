import AddIcon from '@mui/icons-material/Add';
import { Box, Button, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { usePodeEditar } from '@/acesso/eu';
import { type Arquivo, useAgente } from '@/catalogo/contratos';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { EmptyState } from '@/shared/ui/EmptyState';
import { ErroComRetry } from '@/shared/ui/ErroComRetry';
import { NomeDialog } from '@/shared/ui/NomeDialog';
import { SectionCard } from '@/shared/ui/SectionCard';
import { PESOS } from '@/theme/tokens';
import { useGestaoDeArquivos } from './arquivosDaSkill';

type AgenteCarregado = NonNullable<ReturnType<typeof useAgente>['data']>['agente'];

export type ContextoDoArquivo = {
  caminhoDoAgente: string;
  editavel: boolean;
  herdado: boolean;
  descartar: boolean;
  onExcluir: () => void;
};

type Props = {
  caminho: string;
  titulo: string;
  ajuda: string;
  rotuloNovo: string;
  mensagemDeErro: string;
  fraseVazia: string;
  tituloDaExclusao: string;
  textoDaExclusao: (caminhoDoArquivo: string) => string;
  tipo: 'regra' | 'documento';
  selecionar: (agente: AgenteCarregado) => { proprios: Arquivo[]; herdados: Arquivo[] };
  renderArquivo: (arquivo: Arquivo, contexto: ContextoDoArquivo) => ReactNode;
};

export function ArquivosDaSkillSecao({
  caminho,
  titulo,
  ajuda,
  rotuloNovo,
  mensagemDeErro,
  fraseVazia,
  tituloDaExclusao,
  textoDaExclusao,
  tipo,
  selecionar,
  renderArquivo,
}: Props): ReactNode {
  const agente = useAgente(caminho);
  const editavel = usePodeEditar('skills.editar_textos');
  const { proprios, herdados } = agente.data
    ? selecionar(agente.data.agente)
    : { proprios: [], herdados: [] };
  const { criar, excluir, emExclusao } = useGestaoDeArquivos(caminho, tipo, proprios);
  const [criando, setCriando] = useState(false);
  const [excluindo, setExcluindo] = useState<string | null>(null);

  const botaoNovo = editavel && (
    <Button
      size="small"
      variant="outlined"
      startIcon={<AddIcon />}
      onClick={() => setCriando(true)}
      sx={{
        height: 34,
        px: 1.75,
        borderColor: 'border.default',
        color: 'text.strong',
        fontWeight: PESOS.medio,
      }}
    >
      {rotuloNovo}
    </Button>
  );

  const contexto = (arquivo: Arquivo, herdado: boolean): ContextoDoArquivo => ({
    caminhoDoAgente: caminho,
    editavel,
    herdado,
    descartar: emExclusao === arquivo.caminho,
    onExcluir: () => setExcluindo(arquivo.caminho),
  });

  return (
    <SectionCard titulo={titulo} ajuda={ajuda} acao={botaoNovo}>
      {agente.isPending && <Typography variant="caption">Carregando…</Typography>}
      {agente.isError && (
        <ErroComRetry mensagem={mensagemDeErro} onTentarDeNovo={() => void agente.refetch()} />
      )}
      {agente.isSuccess && proprios.length + herdados.length === 0 && (
        <EmptyState frase={fraseVazia} />
      )}
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
        {proprios.map((arquivo) => renderArquivo(arquivo, contexto(arquivo, false)))}
        {herdados.map((arquivo) => renderArquivo(arquivo, contexto(arquivo, true)))}
      </Box>
      {criando && (
        <NomeDialog
          titulo={rotuloNovo}
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
        titulo={tituloDaExclusao}
        texto={textoDaExclusao(excluindo ?? '')}
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
