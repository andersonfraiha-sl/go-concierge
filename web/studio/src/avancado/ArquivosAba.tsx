import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  MenuItem,
  Paper,
  TextField,
  Typography,
} from '@mui/material';
import { type ReactNode, useState } from 'react';
import {
  type Arquivo,
  excluirArquivo,
  lerModeloDeArquivo,
  type Problema,
  salvarArquivo,
  useAgente,
  useArquivo,
} from '@/catalogo/contratos';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { SectionCard } from '@/shared/ui/SectionCard';
import {
  ARQUIVO_DE_CASO,
  ARQUIVO_DE_IDENTIDADE,
  ARQUIVO_DE_PERSONA,
  caminhoNaPasta,
  mensagemDeErro,
  type NoDaArvore,
  PASTA_INICIAL,
  PASTAS_DE_ARQUIVOS,
  type PastaDeArquivos,
  SUGESTAO_DE_NOME,
  useRecarregarAvancado,
} from './contratos';
import { ESTILO_DE_CODIGO } from './estilo';

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

type EditorProps = { agente: string; caminho: string; editar: boolean; onExcluido: () => void };

function EditorDeArquivo(props: EditorProps): ReactNode {
  const arquivo = useArquivo(props.agente, props.caminho);
  if (arquivo.isPending) return <Typography color="text.secondary">Carregando…</Typography>;
  if (arquivo.isError) {
    return (
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Typography role="alert">Não foi possível abrir o arquivo.</Typography>
        <Button size="small" onClick={() => void arquivo.refetch()}>
          Tentar de novo
        </Button>
      </Box>
    );
  }
  return <EditorCarregado {...props} conteudoInicial={arquivo.data.conteudo} />;
}

function EditorCarregado({
  agente,
  caminho,
  editar,
  onExcluido,
  conteudoInicial,
}: EditorProps & { conteudoInicial: string }): ReactNode {
  const { mostrar } = useSnackbar();
  const recarregar = useRecarregarAvancado();
  const [texto, setTexto] = useState(conteudoInicial);
  const [excluindo, setExcluindo] = useState(false);

  const salvar = async (): Promise<void> => {
    try {
      await salvarArquivo({ agente, caminho, conteudo: texto });
      await recarregar();
      mostrar(`Arquivo ${caminho} salvo`);
    } catch (erro) {
      mostrar(mensagemDeErro(erro), 'error');
    }
  };

  const excluir = async (): Promise<void> => {
    setExcluindo(false);
    try {
      await excluirArquivo(agente, caminho);
      await recarregar();
      onExcluido();
      mostrar(`Arquivo ${caminho} excluído`);
    } catch (erro) {
      mostrar(mensagemDeErro(erro), 'error');
    }
  };

  return (
    <SectionCard
      titulo={caminho}
      ajuda="O texto exatamente como está gravado."
      acao={
        editar && (
          <Box sx={{ display: 'flex', gap: 1 }}>
            <Button color="error" onClick={() => setExcluindo(true)}>
              Excluir
            </Button>
            <Button variant="contained" onClick={() => void salvar()}>
              Salvar
            </Button>
          </Box>
        )
      }
    >
      <TextField
        multiline
        minRows={16}
        fullWidth
        value={texto}
        onChange={(evento) => setTexto(evento.target.value)}
        slotProps={{
          htmlInput: {
            readOnly: !editar,
            'aria-label': `Conteúdo de ${caminho}`,
            style: ESTILO_DE_CODIGO,
          },
        }}
      />
      <ConfirmDialog
        aberto={excluindo}
        titulo={`Excluir ${caminho}?`}
        texto="O arquivo sai deste item e não dá para desfazer."
        confirmar="Excluir"
        onConfirmar={() => void excluir()}
        onCancelar={() => setExcluindo(false)}
      />
    </SectionCard>
  );
}

function Problemas({ problemas }: { problemas: Problema[] }): ReactNode {
  return (
    <SectionCard titulo="Problemas" ajuda="O que o servidor achou de errado neste item.">
      {problemas.length === 0 && (
        <Typography variant="body2">Nenhum problema encontrado.</Typography>
      )}
      {problemas.map((problema) => (
        <Box key={`${problema.onde}-${problema.texto}`}>
          <Typography variant="body2" color={problema.nivel === 'erro' ? 'error' : 'text.primary'}>
            {problema.nivel === 'erro' ? 'Erro' : 'Aviso'} em {problema.onde}
          </Typography>
          <Typography variant="body2">{problema.texto}</Typography>
          {problema.solucao && (
            <Typography variant="caption" color="text.secondary">
              {problema.solucao}
            </Typography>
          )}
        </Box>
      ))}
    </SectionCard>
  );
}

type NovoProps = {
  aberto: boolean;
  onCriar: (caminho: string) => Promise<void>;
  onFechar: () => void;
};

function NovoArquivoDialog({ aberto, onCriar, onFechar }: NovoProps): ReactNode {
  const [pasta, setPasta] = useState<PastaDeArquivos>(PASTA_INICIAL);
  const [nome, setNome] = useState(SUGESTAO_DE_NOME[PASTA_INICIAL]);

  const escolherPasta = (valor: string): void => {
    const escolhida = PASTAS_DE_ARQUIVOS.find((p) => p === valor);
    if (!escolhida) return;
    setPasta(escolhida);
    setNome(SUGESTAO_DE_NOME[escolhida]);
  };

  return (
    <Dialog open={aberto} onClose={onFechar} fullWidth maxWidth="xs">
      <DialogTitle>Novo arquivo</DialogTitle>
      <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 1 }}>
        <TextField
          select
          size="small"
          label="Pasta"
          value={pasta}
          onChange={(evento) => escolherPasta(evento.target.value)}
        >
          {PASTAS_DE_ARQUIVOS.map((p) => (
            <MenuItem key={p} value={p}>
              {p}
            </MenuItem>
          ))}
        </TextField>
        <TextField
          size="small"
          label="Nome do arquivo"
          value={nome}
          onChange={(evento) => setNome(evento.target.value)}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onFechar}>Cancelar</Button>
        <Button
          variant="contained"
          disabled={nome.trim() === ''}
          onClick={() => void onCriar(caminhoNaPasta(pasta, nome.trim()))}
        >
          Criar
        </Button>
      </DialogActions>
    </Dialog>
  );
}
