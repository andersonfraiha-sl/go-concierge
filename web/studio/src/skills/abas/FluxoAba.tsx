import LockOutlinedIcon from '@mui/icons-material/LockOutlined';
import { Alert, Box, Button, CircularProgress, Typography } from '@mui/material';
import { type QueryClient, useQueryClient } from '@tanstack/react-query';
import { type Dispatch, type ReactNode, type SetStateAction, useMemo, useState } from 'react';
import { usePodeEditar } from '@/acesso/eu';
import { AvisoDeConflito } from '@/copiloto/AvisoDeConflito';
import { useCopiloto } from '@/copiloto/CopilotoProvider';
import { useContextoDaTela } from '@/copiloto/useContextoDaTela';
import { useEdicaoSobOServidor } from '@/copiloto/useEdicaoSobOServidor';
import { tituloDe } from '@/ferramentas/contratos';
import { FluxoCanvas } from '@/fluxo/canvas/FluxoCanvas';
import { novoBloco } from '@/fluxo/modelo/catalogo';
import { inserirBloco, moverBloco, removerBloco } from '@/fluxo/modelo/edicao';
import type { Fluxo, FluxoLido, ProblemaFluxo, TipoBloco } from '@/fluxo/modelo/tipos';
import { validarFluxo } from '@/fluxo/modelo/validar';
import type { CatalogoDeFerramentas } from '@/fluxo/painel/camposPorTipo';
import { ApiError } from '@/shared/api/http';
import { useAutosave } from '@/shared/autosave/useAutosave';
import { EmptyState } from '@/shared/ui/EmptyState';
import { ESPACOS, RAIO } from '@/theme/tokens';
import { chaveDoFluxo, salvarFluxo, useFluxo } from '../contratos';
import { BarraDoFluxo } from './BarraDoFluxo';
import {
  blocosRealcados,
  juntarProblemas,
  novoId,
  problemasDaRecusa,
  useCatalogoDeFerramentas,
} from './fluxoDaSkill';
import { PainelLateralDoFluxo } from './PainelLateralDoFluxo';

type Props = { nome: string; caminho: string };

// A área acima do canvas não é mais o navbar global: é o CabecalhoDeDetalhe (faixa + abas) da
// própria moldura de detalhe, mais o padding vertical de ESTILO_DO_CONTEUDO_DE_DETALHE.
const ALTURA_DO_CABECALHO_DE_DETALHE = 59;
const ALTURA_DAS_ABAS = 46;
const ALTURA_DO_PADDING_DO_CONTEUDO = ESPACOS[4] * 2;
const ALTURA_DA_AREA = `calc(100vh - ${
  ALTURA_DO_CABECALHO_DE_DETALHE + ALTURA_DAS_ABAS + ALTURA_DO_PADDING_DO_CONTEUDO
}px)`;
const ALTURA_MINIMA_DA_AREA = 480;

function ehSemFluxo(erro: Error): boolean {
  return erro instanceof ApiError && erro.status === 404 && erro.codigo === 'SEM_FLUXO';
}

export function FluxoAba({ nome }: Props): ReactNode {
  const lido = useFluxo(nome);
  useContextoDaTela({ tela: 'fluxo', skill: nome });
  if (lido.isPending) return <CircularProgress aria-label="Carregando fluxo" />;
  if (lido.isError)
    return <FalhaDoFluxo erro={lido.error} onTentarDeNovo={() => void lido.refetch()} />;
  return <EditorDeFluxo key={nome} nome={nome} lido={lido.data} />;
}

/** Faixa no topo do canvas quando o gestor só pode ver o passo a passo, não editá-lo. */
function AvisoDeSoLeitura(): ReactNode {
  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 1.5,
        mb: 2.5,
        py: 1.25,
        px: 2,
        bgcolor: 'background.paper',
        border: 1,
        borderColor: 'divider',
        borderRadius: RAIO.painel,
      }}
    >
      <LockOutlinedIcon fontSize="small" sx={{ color: 'text.secondary' }} />
      <Typography variant="body2" sx={{ color: 'text.strong' }}>
        Você vê o passo a passo como gestor. Para mudar, fale com quem constrói as skills ou peça ao
        Copiloto uma explicação.
      </Typography>
    </Box>
  );
}

function FalhaDoFluxo({ erro, onTentarDeNovo }: { erro: Error; onTentarDeNovo: () => void }) {
  if (ehSemFluxo(erro)) {
    return <EmptyState frase="Esta skill não tem fluxo: ela responde pela base de conhecimento." />;
  }
  return (
    <Alert severity="error" action={<Button onClick={onTentarDeNovo}>Tentar de novo</Button>}>
      {erro.message}
    </Alert>
  );
}

type EditorProps = { nome: string; lido: FluxoLido };

function EditorDeFluxo({ nome, lido }: EditorProps): ReactNode {
  const [fluxo, setFluxo] = useState<Fluxo>(lido.fluxo);
  const editavel = usePodeEditar('skills.editar_fluxo');
  const catalogo = useCatalogoDeFerramentas();
  const { locais, problemas, setProblemasDoServidor } = useProblemasDoFluxo(fluxo, lido.problemas);
  const salvamento = useSalvamentoDoFluxo({
    nome,
    lido,
    fluxo,
    locais,
    setFluxo,
    setProblemasDoServidor,
  });
  const edicao = useEdicaoDosBlocos(fluxo, setFluxo);

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        gap: 1,
        height: ALTURA_DA_AREA,
        minHeight: ALTURA_MINIMA_DA_AREA,
      }}
    >
      <AvisoDeConflito
        conflito={salvamento.sincronia.conflito}
        onManterOMeu={salvamento.sincronia.manterOMeu}
        onUsarONovo={salvamento.sincronia.usarONovo}
      />
      <BarraDoFluxo
        editavel={editavel}
        totalDePassos={fluxo.blocos.length}
        problemas={problemas}
        doFluxoTodo={problemas.filter((p) => p.blocoId === undefined)}
        salvamento={salvamento.estado}
        onTentarDeNovo={() => void salvamento.tentarDeNovo()}
      />
      <CanvasEPainel
        nome={nome}
        fluxo={fluxo}
        problemas={problemas}
        catalogo={catalogo}
        editavel={editavel}
        edicao={edicao}
        onMudarFluxo={setFluxo}
      />
    </Box>
  );
}

function useProblemasDoFluxo(fluxo: Fluxo, doLido: FluxoLido['problemas']) {
  const [problemasDoServidor, setProblemasDoServidor] = useState(doLido);
  const locais = useMemo(() => validarFluxo(fluxo), [fluxo]);
  const problemas = useMemo(
    () => juntarProblemas(locais, problemasDoServidor, fluxo.blocos),
    [locais, problemasDoServidor, fluxo.blocos],
  );
  return { locais, problemas, setProblemasDoServidor };
}

type PropsDoSalvamento = {
  nome: string;
  lido: FluxoLido;
  fluxo: Fluxo;
  locais: readonly ProblemaFluxo[];
  setFluxo: (fluxo: Fluxo) => void;
  setProblemasDoServidor: (problemas: FluxoLido['problemas']) => void;
};

function useSalvamentoDoFluxo(props: PropsDoSalvamento) {
  const { nome, lido, fluxo, locais, setFluxo, setProblemasDoServidor } = props;
  const cliente = useQueryClient();
  const sincronia = useEdicaoSobOServidor({
    servidor: lido.fluxo,
    alvos: [`skill:${nome}:fluxo`],
    versao: lido.versao,
    local: fluxo,
    resetar: setFluxo,
    salvar: (valor, versao) =>
      gravarFluxo({ nome, valor, versao, cliente, aoMudarProblemas: setProblemasDoServidor }),
  });
  const { estado, tentarDeNovo } = useAutosave({
    valor: fluxo,
    bloqueio: sincronia.bloqueio ?? (locais.length > 0 ? 'corrija o bloco marcado' : null),
    salvar: sincronia.salvar,
  });
  return { sincronia, estado, tentarDeNovo };
}

type Gravacao = {
  nome: string;
  valor: Fluxo;
  versao: string;
  cliente: QueryClient;
  aoMudarProblemas: (problemas: FluxoLido['problemas']) => void;
};

async function gravarFluxo({ nome, valor, versao, cliente, aoMudarProblemas }: Gravacao) {
  try {
    const gravado = await salvarFluxo(nome, { versao, fluxo: valor });
    aoMudarProblemas(gravado.problemas);
    // A aba sai da tela ao trocar de aba; ao voltar, o cache precisa ter a versão e o fluxo gravados.
    cliente.setQueryData<FluxoLido>(chaveDoFluxo(nome), {
      versao: gravado.versao,
      fluxo: valor,
      problemas: gravado.problemas,
    });
    return gravado.versao;
  } catch (erro) {
    const recusados = problemasDaRecusa(erro);
    if (recusados !== null) aoMudarProblemas(recusados);
    throw erro;
  }
}

function useEdicaoDosBlocos(fluxo: Fluxo, setFluxo: Dispatch<SetStateAction<Fluxo>>) {
  const [selecionado, setSelecionado] = useState<string | null>(null);
  const inserir = (indice: number, tipo: TipoBloco) => {
    const id = novoId(fluxo.blocos);
    setFluxo((atual) => inserirBloco(atual, indice, novoBloco(tipo, id)));
    setSelecionado(id);
  };
  const excluir = (id: string) => {
    setFluxo((atual) => removerBloco(atual, id));
    setSelecionado(null);
  };
  return { selecionado, setSelecionado, inserir, excluir };
}

function useTituloDaFerramenta(catalogo: CatalogoDeFerramentas) {
  const ferramentas = catalogo.estado === 'pronto' ? catalogo.ferramentas : null;
  return useMemo(() => {
    const titulos = new Map((ferramentas ?? []).map((f) => [f.name, tituloDe(f)]));
    return (recurso: string): string => titulos.get(recurso) ?? recurso;
  }, [ferramentas]);
}

/** Nome amigável de uma informação pedida ao morador, pelo `titulo` gravado no fluxo. */
function useTituloDaInformacao(fluxo: Fluxo) {
  return useMemo(() => {
    const titulos = new Map(fluxo.informacoes.map((info) => [info.nome, info.titulo]));
    return (nome: string): string => {
      const titulo = titulos.get(nome);
      return titulo && titulo !== '' ? titulo : nome;
    };
  }, [fluxo.informacoes]);
}

type PropsDoCanvas = {
  nome: string;
  fluxo: Fluxo;
  problemas: ProblemaFluxo[];
  catalogo: CatalogoDeFerramentas;
  editavel: boolean;
  edicao: ReturnType<typeof useEdicaoDosBlocos>;
  onMudarFluxo: Dispatch<SetStateAction<Fluxo>>;
};

// por que: monta canvas e painel lado a lado; o que tem de lógica já saiu para os hooks acima.
function CanvasEPainel(props: PropsDoCanvas): ReactNode {
  const { nome, fluxo, problemas, catalogo, editavel, edicao, onMudarFluxo } = props;
  const { alvosRealcados } = useCopiloto();
  const realcados = useMemo(() => blocosRealcados(alvosRealcados, nome), [alvosRealcados, nome]);
  const tituloDaFerramenta = useTituloDaFerramenta(catalogo);
  const tituloDaInformacao = useTituloDaInformacao(fluxo);
  const blocoSelecionado = fluxo.blocos.find((b) => b.id === edicao.selecionado);

  return (
    <Box
      sx={{
        display: 'flex',
        flex: 1,
        minHeight: 0,
        border: 1,
        borderColor: 'divider',
        borderRadius: RAIO.campo,
        overflow: 'hidden',
      }}
    >
      <Box sx={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
        {!editavel && <AvisoDeSoLeitura />}
        <FluxoCanvas
          fluxo={fluxo}
          problemas={problemas}
          selecionado={edicao.selecionado}
          editavel={editavel}
          tituloDaFerramenta={tituloDaFerramenta}
          tituloDaInformacao={tituloDaInformacao}
          realcados={realcados}
          onSelecionar={edicao.setSelecionado}
          onInserir={edicao.inserir}
          onMover={(de, para) => onMudarFluxo((atual) => moverBloco(atual, de, para))}
        />
      </Box>
      <PainelLateralDoFluxo
        fluxo={fluxo}
        bloco={blocoSelecionado}
        problemas={problemas}
        catalogo={catalogo}
        editavel={editavel}
        onMudarFluxo={onMudarFluxo}
        onExcluir={edicao.excluir}
      />
    </Box>
  );
}
