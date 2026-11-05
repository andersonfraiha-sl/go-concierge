import AddIcon from '@mui/icons-material/Add';
import { Box, Button, Typography } from '@mui/material';
import { type ReactNode, useEffect, useState } from 'react';
import { pode, useEu } from '@/acesso/eu';
import { type Arquivo, lerModeloDeArquivo, salvarArquivo, useAgente } from '@/catalogo/contratos';
import { type EstiloDoRealce, useCopiloto, useRealceDoCopiloto } from '@/copiloto/CopilotoProvider';
import { useContextoDaTela } from '@/copiloto/useContextoDaTela';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import {
  LARGURA_DA_LISTA_ESTREITA,
  PESOS,
  RAIO,
  RAIO_DO_ITEM_DA_ARVORE,
  TIPOS,
} from '@/theme/tokens';
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
import { FalhaComTentarDeNovo } from './FalhaComTentarDeNovo';
import { type ArquivoNaArvore, agruparArquivos, type GrupoDeArquivos } from './gruposDeArquivos';
import { NovoArquivoDialog } from './NovoArquivoDialog';

type Props = { no: NoDaArvore; editar: boolean };

/** Abre o primeiro arquivo da árvore sozinho quando nada está selecionado ainda. */
function usePrimeiroArquivoAberto(
  grupos: readonly GrupoDeArquivos[],
  aberto: string | null,
  setAberto: (caminho: string) => void,
): void {
  useEffect(() => {
    if (aberto !== null) return;
    const primeiro = grupos[0]?.arquivos[0];
    if (primeiro) setAberto(primeiro.caminho);
  }, [grupos, aberto, setAberto]);
}

// Lista e editor vão até o pé da janela (topo, título, abas e seletor somam ~380 px).
const ALTURA_DA_AREA = 'max(480px, calc(100vh - 380px))';

export function ArquivosAba({ no, editar }: Props): ReactNode {
  const agente = useAgente(no.caminho);
  const [aberto, setAberto] = useState<string | null>(null);
  const [criando, setCriando] = useState(false);
  const criar = useCriarArquivo(no.caminho, (caminho) => {
    setCriando(false);
    setAberto(caminho);
  });
  useContextoDoArquivo(no.caminho, aberto);
  const realce = useRealceDoArquivo(no.caminho, aberto);
  const copilotoAberto = useCopiloto().aberto;

  const grupos = agruparArquivos(
    agente.data
      ? montarArquivosPorPasta(
          no,
          agente.data.agente.persona !== '',
          agente.data.agente.arquivos,
          agente.data.agente.herdados,
        )
      : {},
  );
  usePrimeiroArquivoAberto(grupos, aberto, setAberto);

  if (agente.isPending) return <Typography color="textSecondary">Carregando…</Typography>;
  if (agente.isError) {
    return (
      <FalhaComTentarDeNovo
        mensagem="Não foi possível carregar os arquivos."
        onTentarDeNovo={() => void agente.refetch()}
      />
    );
  }

  return (
    <Box sx={{ display: 'flex', gap: 3, alignItems: 'stretch', height: ALTURA_DA_AREA }}>
      <ListaDeGrupos
        grupos={grupos}
        aberto={aberto}
        editar={editar && !copilotoAberto}
        onAbrir={setAberto}
        onNovo={() => setCriando(true)}
      />
      <Box sx={{ flex: 1, minWidth: 320, display: 'flex', '& .MuiInputBase-root': realce }}>
        {aberto ? (
          <EditorDeArquivo
            key={aberto}
            agente={no.caminho}
            caminho={aberto}
            editar={editar}
            onExcluido={() => setAberto(null)}
          />
        ) : (
          <Typography color="textSecondary">Escolha um arquivo na lista para abrir.</Typography>
        )}
      </Box>
      <NovoArquivoDialog aberto={criando} onCriar={criar} onFechar={() => setCriando(false)} />
    </Box>
  );
}

function useContextoDoArquivo(agente: string, aberto: string | null): void {
  const { data: eu } = useEu();
  const podeVerArquivos = eu !== undefined && pode(eu, 'studio.avancado');
  useContextoDaTela(
    podeVerArquivos
      ? { tela: 'avancado', agente, ...(aberto !== null && { caminho: aberto }) }
      : null,
  );
}

function useCriarArquivo(
  agente: string,
  onCriado: (caminho: string) => void,
): (caminho: string) => Promise<void> {
  const { mostrar } = useSnackbar();
  const recarregar = useRecarregarAvancado();
  return async (caminho) => {
    try {
      await salvarArquivo({ agente, caminho, conteudo: await lerModeloDeArquivo(caminho) });
      await recarregar();
    } catch (erro) {
      mostrar(mensagemDeErro(erro), 'error');
      return;
    }
    onCriado(caminho);
    mostrar(`Arquivo ${caminho} criado`);
  };
}

function useRealceDoArquivo(agente: string, aberto: string | null): EstiloDoRealce {
  const realceDoAgente = useRealceDoCopiloto(`arquivo:${agente}:${aberto ?? ''}`);
  // Proposta sem agente vale para o agente desta tela: o servidor só aceita a do contexto.
  const realceSemAgente = useRealceDoCopiloto(`arquivo::${aberto ?? ''}`);
  return realceSemAgente.bgcolor === undefined ? realceDoAgente : realceSemAgente;
}

type ListaDeGruposProps = {
  grupos: GrupoDeArquivos[];
  aberto: string | null;
  editar: boolean;
  onAbrir: (caminho: string) => void;
  onNovo: () => void;
};

function ListaDeGrupos({ grupos, aberto, editar, onAbrir, onNovo }: ListaDeGruposProps): ReactNode {
  return (
    <Box
      sx={{
        width: LARGURA_DA_LISTA_ESTREITA,
        flexShrink: 0,
        display: 'flex',
        flexDirection: 'column',
        gap: 1,
      }}
    >
      <Box
        sx={{
          flex: 1,
          minHeight: 0,
          overflowY: 'auto',
          display: 'flex',
          flexDirection: 'column',
          gap: 0.25,
        }}
      >
        {grupos.map((grupo) => (
          <ListaDoGrupo key={grupo.grupo} grupo={grupo} aberto={aberto} onAbrir={onAbrir} />
        ))}
      </Box>
      {editar && (
        <Button
          startIcon={<AddIcon fontSize="small" />}
          onClick={onNovo}
          sx={{
            flexShrink: 0,
            justifyContent: 'center',
            borderWidth: 1,
            borderStyle: 'dashed',
            borderColor: 'text.faint',
            borderRadius: RAIO.chip,
            color: 'text.secondary',
            fontWeight: PESOS.medio,
          }}
        >
          Novo arquivo
        </Button>
      )}
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

const rotuloDoArquivo = (arquivo: ArquivoNaArvore): string =>
  arquivo.herdado_de ? `${arquivo.exibicao} (herdado de ${arquivo.herdado_de})` : arquivo.exibicao;

type ListaProps = {
  grupo: GrupoDeArquivos;
  aberto: string | null;
  onAbrir: (caminho: string) => void;
};

const ESTILO_DO_TITULO_DO_GRUPO = {
  pt: 2,
  pb: 0.5,
  px: 1.25,
  fontSize: TIPOS.overline,
  fontWeight: PESOS.semi,
  color: 'text.muted',
  textTransform: 'uppercase',
  letterSpacing: '.06em',
} as const;

function ListaDoGrupo({ grupo, aberto, onAbrir }: ListaProps): ReactNode {
  return (
    <Box>
      <Typography sx={ESTILO_DO_TITULO_DO_GRUPO}>{grupo.grupo}</Typography>
      {grupo.arquivos.map((arquivo) => (
        <ItemDoGrupo key={arquivo.caminho} arquivo={arquivo} aberto={aberto} onAbrir={onAbrir} />
      ))}
    </Box>
  );
}

type ItemProps = {
  arquivo: ArquivoNaArvore;
  aberto: string | null;
  onAbrir: (caminho: string) => void;
};

function ItemDoGrupo({ arquivo, aberto, onAbrir }: ItemProps): ReactNode {
  if (arquivo.herdado_de) {
    return (
      <Typography variant="body2" color="textSecondary" sx={{ py: 0.5, px: 1.25 }}>
        {rotuloDoArquivo(arquivo)}
      </Typography>
    );
  }
  const selecionado = aberto === arquivo.caminho;
  return (
    <Button
      fullWidth
      size="small"
      color="inherit"
      sx={{
        justifyContent: 'flex-start',
        textTransform: 'none',
        fontFamily: ESTILO_DE_CODIGO.fontFamily,
        fontSize: TIPOS.caption,
        fontWeight: PESOS.regular,
        borderRadius: RAIO_DO_ITEM_DA_ARVORE,
        py: 0.875,
        px: 1.25,
        ...(selecionado
          ? { bgcolor: 'background.tint', color: 'primary.main' }
          : { color: 'text.strong' }),
      }}
      onClick={() => onAbrir(arquivo.caminho)}
    >
      {arquivo.exibicao}
    </Button>
  );
}
