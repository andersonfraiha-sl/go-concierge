import { Box, Button, Paper, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { type Arquivo, lerModeloDeArquivo, salvarArquivo, useAgente } from '@/catalogo/contratos';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
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
import { NovoArquivoDialog } from './NovoArquivoDialog';
import { Problemas } from './ProblemasDoItem';

type Props = { no: NoDaArvore; editar: boolean };

type Entrada = { caminho: string; rotulo?: string; herdado?: boolean };
type Grupo = { pasta: string; entradas: Entrada[] };

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
  const grupos = montarGrupos(no, persona !== '', arquivos, herdados);

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
            <ListaDaPasta key={grupo.pasta} grupo={grupo} aberto={aberto} onAbrir={setAberto} />
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

function montarGrupos(
  no: NoDaArvore,
  temPersona: boolean,
  arquivos: Record<string, Arquivo[]>,
  herdados: Record<string, Arquivo[]>,
): Grupo[] {
  const raiz: Entrada[] = [
    { caminho: ARQUIVO_DE_IDENTIDADE },
    ...(temPersona ? [{ caminho: ARQUIVO_DE_PERSONA }] : []),
    ...(no.caso ? [{ caminho: ARQUIVO_DE_CASO }] : []),
  ];
  const pastas = new Set([...Object.keys(arquivos), ...Object.keys(herdados)]);
  const dasPastas = [...pastas].map((pasta) => ({
    pasta,
    entradas: [
      ...(arquivos[pasta] ?? []).map((a) => ({ caminho: a.caminho })),
      ...(herdados[pasta] ?? []).map((a) => ({
        caminho: a.caminho,
        herdado: true,
        rotulo: `${a.caminho} (herdado de ${a.herdado_de ?? 'outro item'})`,
      })),
    ],
  }));
  return [{ pasta: 'Raiz', entradas: raiz }, ...dasPastas].filter((g) => g.entradas.length > 0);
}

type ListaProps = { grupo: Grupo; aberto: string | null; onAbrir: (caminho: string) => void };

function ListaDaPasta({ grupo, aberto, onAbrir }: ListaProps): ReactNode {
  return (
    <Box>
      <Typography variant="caption" color="text.secondary">
        {grupo.pasta}
      </Typography>
      {grupo.entradas.map((entrada) =>
        entrada.herdado ? (
          <Typography key={entrada.caminho} variant="body2" color="text.secondary" sx={{ py: 0.5 }}>
            {entrada.rotulo}
          </Typography>
        ) : (
          <Button
            key={entrada.caminho}
            fullWidth
            size="small"
            color="inherit"
            variant={aberto === entrada.caminho ? 'outlined' : 'text'}
            sx={{
              justifyContent: 'flex-start',
              textTransform: 'none',
              fontFamily: ESTILO_DE_CODIGO.fontFamily,
            }}
            onClick={() => onAbrir(entrada.caminho)}
          >
            {entrada.caminho}
          </Button>
        ),
      )}
    </Box>
  );
}
