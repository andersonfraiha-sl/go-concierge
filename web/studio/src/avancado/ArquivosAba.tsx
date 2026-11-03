import { Box, Button, Paper, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { type Arquivo, lerModeloDeArquivo, salvarArquivo, useAgente } from '@/catalogo/contratos';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { TIPOS, tokens } from '@/theme/tokens';
import {
  ARQUIVO_DE_CASO,
  ARQUIVO_DE_IDENTIDADE,
  ARQUIVO_DE_PERSONA,
  mensagemDeErro,
  type NoDaArvore,
  useRecarregarAvancado,
} from './contratos';
import { EditorDeArquivo } from './EditorDeArquivo';
import { ESTILO_DE_CODIGO } from './estilo';
import { agruparArquivos, type GrupoDeArquivos } from './gruposDeArquivos';
import { NovoArquivoDialog } from './NovoArquivoDialog';
import { Problemas } from './ProblemasDoItem';

type Props = { no: NoDaArvore; editar: boolean };

export function ArquivosAba({ no, editar }: Props): ReactNode {
  const agente = useAgente(no.caminho);
  const [aberto, setAberto] = useState<string | null>(null);
  const [criando, setCriando] = useState(false);
  const { mostrar } = useSnackbar();
  const recarregar = useRecarregarAvancado();

  if (agente.isPending) return <Typography color="text.secondary">Carregando…</Typography>;
  if (agente.isError) {
    return (
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Typography role="alert">Não foi possível carregar os arquivos.</Typography>
        <Button size="small" onClick={() => void agente.refetch()}>
          Tentar de novo
        </Button>
      </Box>
    );
  }

  const { arquivos, herdados, persona, problemas } = agente.data.agente;
  const grupos = agruparArquivos(montarArquivosPorPasta(no, persona !== '', arquivos, herdados));

  const criar = async (caminho: string): Promise<void> => {
    try {
      await salvarArquivo({
        agente: no.caminho,
        caminho,
        conteudo: await lerModeloDeArquivo(caminho),
      });
      await recarregar();
      setCriando(false);
      setAberto(caminho);
      mostrar(`Arquivo ${caminho} criado`);
    } catch (erro) {
      mostrar(mensagemDeErro(erro), 'error');
    }
  };

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <Box sx={{ display: 'flex', gap: 2, alignItems: 'flex-start', flexWrap: 'wrap' }}>
        <Paper
          variant="outlined"
          sx={{ p: 1.5, width: 300, display: 'flex', flexDirection: 'column', gap: 1 }}
        >
          {editar && (
            <Button variant="outlined" size="small" onClick={() => setCriando(true)}>
              Novo arquivo
            </Button>
          )}
          {grupos.map((grupo) => (
            <ListaDoGrupo key={grupo.grupo} grupo={grupo} aberto={aberto} onAbrir={setAberto} />
          ))}
        </Paper>
        <Box sx={{ flex: 1, minWidth: 320 }}>
          {aberto ? (
            <EditorDeArquivo
              key={aberto}
              agente={no.caminho}
              caminho={aberto}
              editar={editar}
              onExcluido={() => setAberto(null)}
            />
          ) : (
            <Typography color="text.secondary">Escolha um arquivo na lista para abrir.</Typography>
          )}
        </Box>
      </Box>
      <Problemas problemas={problemas} />
      <NovoArquivoDialog aberto={criando} onCriar={criar} onFechar={() => setCriando(false)} />
    </Box>
  );
}

/** Arquivos (reais e herdados) de cada pasta, mais os três itens virtuais da raiz. */
function montarArquivosPorPasta(
  no: NoDaArvore,
  temPersona: boolean,
  arquivos: Record<string, Arquivo[]>,
  herdados: Record<string, Arquivo[]>,
): Record<string, Arquivo[]> {
  const pastas = new Set([...Object.keys(arquivos), ...Object.keys(herdados)]);
  const dasPastas = Object.fromEntries(
    [...pastas].map((pasta) => [pasta, [...(arquivos[pasta] ?? []), ...(herdados[pasta] ?? [])]]),
  );
  return {
    'agent.yaml': [{ caminho: ARQUIVO_DE_IDENTIDADE, bytes: 0 }],
    ...(temPersona ? { 'persona.md': [{ caminho: ARQUIVO_DE_PERSONA, bytes: 0 }] } : {}),
    ...(no.caso ? { 'caso.yaml': [{ caminho: ARQUIVO_DE_CASO, bytes: 0 }] } : {}),
    ...dasPastas,
  };
}

const rotuloDoArquivo = (arquivo: Arquivo): string =>
  arquivo.herdado_de ? `${arquivo.caminho} (herdado de ${arquivo.herdado_de})` : arquivo.caminho;

type ListaProps = {
  grupo: GrupoDeArquivos;
  aberto: string | null;
  onAbrir: (caminho: string) => void;
};

function ListaDoGrupo({ grupo, aberto, onAbrir }: ListaProps): ReactNode {
  return (
    <Box>
      <Typography
        sx={{
          fontSize: TIPOS.overline,
          fontWeight: 600,
          color: tokens.color.text.muted,
          textTransform: 'uppercase',
          letterSpacing: '.06em',
        }}
      >
        {grupo.grupo}
      </Typography>
      {grupo.arquivos.map((arquivo) =>
        arquivo.herdado_de ? (
          <Typography key={arquivo.caminho} variant="body2" color="text.secondary" sx={{ py: 0.5 }}>
            {rotuloDoArquivo(arquivo)}
          </Typography>
        ) : (
          <Button
            key={arquivo.caminho}
            fullWidth
            size="small"
            color="inherit"
            variant={aberto === arquivo.caminho ? 'outlined' : 'text'}
            sx={{
              justifyContent: 'flex-start',
              textTransform: 'none',
              fontFamily: ESTILO_DE_CODIGO.fontFamily,
              fontSize: TIPOS.caption,
            }}
            onClick={() => onAbrir(arquivo.caminho)}
          >
            {arquivo.caminho}
          </Button>
        ),
      )}
    </Box>
  );
}
