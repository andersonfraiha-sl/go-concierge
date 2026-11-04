import { Box, Button, Paper, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { pode, useEu } from '@/acesso/eu';
import { type Arquivo, lerModeloDeArquivo, salvarArquivo, useAgente } from '@/catalogo/contratos';
import { type EstiloDoRealce, useRealceDoCopiloto } from '@/copiloto/CopilotoProvider';
import { useContextoDaTela } from '@/copiloto/useContextoDaTela';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { PESOS, TIPOS } from '@/theme/tokens';
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
import { agruparArquivos, type GrupoDeArquivos } from './gruposDeArquivos';
import { NovoArquivoDialog } from './NovoArquivoDialog';
import { Problemas } from './ProblemasDoItem';

type Props = { no: NoDaArvore; editar: boolean };

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

  if (agente.isPending) return <Typography color="text.secondary">Carregando…</Typography>;
  if (agente.isError) {
    return (
      <FalhaComTentarDeNovo
        mensagem="Não foi possível carregar os arquivos."
        onTentarDeNovo={() => void agente.refetch()}
      />
    );
  }

  const { arquivos, herdados, persona, problemas } = agente.data.agente;
  const grupos = agruparArquivos(montarArquivosPorPasta(no, persona !== '', arquivos, herdados));

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <Box sx={{ display: 'flex', gap: 2, alignItems: 'flex-start', flexWrap: 'wrap' }}>
        <ListaDeGrupos
          grupos={grupos}
          aberto={aberto}
          editar={editar}
          onAbrir={setAberto}
          onNovo={() => setCriando(true)}
        />
        <Box sx={{ flex: 1, minWidth: 320, '& .MuiInputBase-root': realce }}>
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
    <Paper
      variant="outlined"
      sx={{ p: 1.5, width: 300, display: 'flex', flexDirection: 'column', gap: 1 }}
    >
      {editar && (
        <Button variant="outlined" size="small" onClick={onNovo}>
          Novo arquivo
        </Button>
      )}
      {grupos.map((grupo) => (
        <ListaDoGrupo key={grupo.grupo} grupo={grupo} aberto={aberto} onAbrir={onAbrir} />
      ))}
    </Paper>
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

const ESTILO_DO_TITULO_DO_GRUPO = {
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

type ItemProps = { arquivo: Arquivo; aberto: string | null; onAbrir: (caminho: string) => void };

function ItemDoGrupo({ arquivo, aberto, onAbrir }: ItemProps): ReactNode {
  if (arquivo.herdado_de) {
    return (
      <Typography variant="body2" color="text.secondary" sx={{ py: 0.5 }}>
        {rotuloDoArquivo(arquivo)}
      </Typography>
    );
  }
  return (
    <Button
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
  );
}
