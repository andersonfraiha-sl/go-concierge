import { useQueryClient } from '@tanstack/react-query';
import {
  createContext,
  type ReactNode,
  type RefObject,
  useContext,
  useEffect,
  useRef,
  useState,
} from 'react';
import { ApiError } from '@/shared/api/http';
import { mensagemDe } from '@/shared/api/mensagemDe';
import { tokens } from '@/theme/tokens';
import {
  CHAVE_DO_ESTADO,
  type ContextoDoCopiloto,
  conversar,
  historicoDe,
  type Proposta,
  useEstadoDoCopiloto,
} from './contratos';
import { gravarAberto, lerAberto } from './preferenciaDoPainel';
import { useAlvosRealcados } from './useAlvosRealcados';
import { type AplicacaoDePropostas, useAplicarPropostas } from './useAplicarPropostas';

export type Status = 'pronto' | 'pensando' | 'indisponivel';

export type Mensagem = {
  id: string;
  papel: 'pessoa' | 'copiloto';
  texto: string;
  propostas: Proposta[];
  tom: 'normal' | 'erro';
};

type ValorDoCopiloto = AplicacaoDePropostas & {
  aberto: boolean;
  alternar: () => void;
  /** Fecha pelo X do painel e devolve o foco ao botão do header, que continua na tela. */
  fechar: () => void;
  botaoDoCopiloto: RefObject<HTMLButtonElement | null>;
  status: Status;
  mensagens: Mensagem[];
  /** Reativo: só muda (e só então redesenha quem o lê) quando a tela de fato troca. */
  contexto: ContextoDoCopiloto;
  enviar: (texto: string) => Promise<void>;
  descartar: (idDaProposta: string) => void;
  definirContexto: (contexto: ContextoDoCopiloto) => void;
  alvosRealcados: ReadonlySet<string>;
  realcar: (alvos: readonly string[]) => void;
};

/** Contexto de quem não diz nada: o mesmo do Studio recém-aberto. */
export const CONTEXTO_NEUTRO: ContextoDoCopiloto = { tela: 'skills' };
const MENSAGEM_DE_DEMORA = 'O Copiloto demorou demais.';

/** Objeto simples (não `SxProps`) para entrar em lista de `sx` junto do estilo do campo. */
export type EstiloDoRealce = { transition: string; bgcolor?: string };

const CopilotoContext = createContext<ValorDoCopiloto | null>(null);

export function useCopiloto(): ValorDoCopiloto {
  const valor = useContext(CopilotoContext);
  if (!valor) throw new Error('useCopiloto fora do CopilotoProvider');
  return valor;
}

/** Fundo de realce por 1500 ms no campo que uma proposta do Copiloto acabou de mudar. */
export function useRealceDoCopiloto(alvo: string): EstiloDoRealce {
  const { alvosRealcados } = useCopiloto();
  const transicao = { transition: 'background-color 300ms ease-out' };
  if (!alvosRealcados.has(alvo)) return transicao;
  return { ...transicao, bgcolor: tokens.color.realceDoCopiloto };
}

type PedidoEmCurso = RefObject<AbortController | null>;

function useAbertura(pedidoEmCurso: PedidoEmCurso) {
  const [aberto, setAberto] = useState(lerAberto);
  const botaoDoCopiloto = useRef<HTMLButtonElement>(null);

  const alternar = () => {
    const proximo = !aberto;
    if (!proximo) pedidoEmCurso.current?.abort();
    setAberto(proximo);
    gravarAberto(proximo);
  };

  const fechar = () => {
    if (aberto) alternar();
    botaoDoCopiloto.current?.focus();
  };

  return { aberto, alternar, fechar, botaoDoCopiloto };
}

function useContextoReativo() {
  // Ref para o pedido em curso (sempre a última tela, mesmo meio a um envio); state para quem
  // lê no render (ex.: sugestões do estado vazio) — só muda, e só então redesenha, quando a
  // tela de fato troca, nunca a cada chamada de definirContexto com o mesmo valor.
  const contextoDoPedido = useRef(CONTEXTO_NEUTRO);
  const [contexto, setContexto] = useState<ContextoDoCopiloto>(CONTEXTO_NEUTRO);

  const definirContexto = (proximo: ContextoDoCopiloto) => {
    contextoDoPedido.current = proximo;
    setContexto((atual) => (contextosIguais(atual, proximo) ? atual : proximo));
  };

  return { contexto, contextoDoPedido, definirContexto };
}

function useConversa(
  pedidoEmCurso: PedidoEmCurso,
  contextoDoPedido: RefObject<ContextoDoCopiloto>,
) {
  const [mensagens, setMensagens] = useState<Mensagem[]>([]);
  const [pensando, setPensando] = useState(false);
  const queryClient = useQueryClient();

  // O id da proposta vem do servidor como índice ("0", "1"…) e se repete entre respostas.
  const acrescentar = (mensagem: Omit<Mensagem, 'id'>) => {
    const id = crypto.randomUUID();
    const propostas = mensagem.propostas.map((proposta) => ({
      ...proposta,
      id: `${id}:${proposta.id}`,
    }));
    setMensagens((atuais) => [...atuais, { ...mensagem, id, propostas }]);
  };

  const registrarFalha = async (erro: unknown) => {
    if (erro instanceof ApiError && erro.codigo === 'COPILOTO_INDISPONIVEL') {
      await queryClient.invalidateQueries({ queryKey: CHAVE_DO_ESTADO });
    }
    acrescentar({ papel: 'copiloto', texto: textoDoErro(erro), propostas: [], tom: 'erro' });
  };

  const enviar = async (texto: string) => {
    const controle = new AbortController();
    pedidoEmCurso.current = controle;
    const pedido = {
      mensagem: texto,
      historico: historicoDe(mensagens.filter((mensagem) => mensagem.tom === 'normal')),
      contexto: contextoDoPedido.current,
    };
    acrescentar({ papel: 'pessoa', texto, propostas: [], tom: 'normal' });
    setPensando(true);
    try {
      const resposta = await conversar(pedido, controle.signal);
      acrescentar({
        papel: 'copiloto',
        texto: resposta.mensagem,
        propostas: resposta.propostas,
        tom: 'normal',
      });
    } catch (erro) {
      if (!controle.signal.aborted) await registrarFalha(erro);
    } finally {
      if (pedidoEmCurso.current === controle) pedidoEmCurso.current = null;
      setPensando(false);
    }
  };

  const descartar = (idDaProposta: string) =>
    setMensagens((atuais) =>
      atuais.map((mensagem) => ({
        ...mensagem,
        propostas: mensagem.propostas.filter((proposta) => proposta.id !== idDaProposta),
      })),
    );

  return { mensagens, pensando, enviar, descartar };
}

export function CopilotoProvider({ children }: { children: ReactNode }) {
  const pedidoEmCurso = useRef<AbortController | null>(null);
  const { aberto, alternar, fechar, botaoDoCopiloto } = useAbertura(pedidoEmCurso);
  const { contexto, contextoDoPedido, definirContexto } = useContextoReativo();
  const { mensagens, pensando, enviar, descartar } = useConversa(pedidoEmCurso, contextoDoPedido);
  const [alvosRealcados, realcar] = useAlvosRealcados();
  const aplicacao = useAplicarPropostas(realcar);
  const estado = useEstadoDoCopiloto(aberto);

  useEffect(() => () => pedidoEmCurso.current?.abort(), []);

  const valor: ValorDoCopiloto = {
    aberto,
    alternar,
    fechar,
    botaoDoCopiloto,
    status: statusDe(estado.data?.disponivel, pensando),
    mensagens,
    contexto,
    enviar,
    ...aplicacao,
    descartar,
    definirContexto,
    alvosRealcados,
    realcar,
  };

  return <CopilotoContext.Provider value={valor}>{children}</CopilotoContext.Provider>;
}

function contextosIguais(a: ContextoDoCopiloto, b: ContextoDoCopiloto): boolean {
  return (
    a.tela === b.tela &&
    a.skill === b.skill &&
    a.condominio === b.condominio &&
    a.agente === b.agente &&
    a.caminho === b.caminho
  );
}

function textoDoErro(erro: unknown): string {
  const estourouOPrazo = erro instanceof ApiError && erro.codigo === 'TEMPO_ESGOTADO';
  return estourouOPrazo ? MENSAGEM_DE_DEMORA : mensagemDe(erro);
}

function statusDe(disponivel: boolean | undefined, pensando: boolean): Status {
  if (disponivel === false) return 'indisponivel';
  return pensando ? 'pensando' : 'pronto';
}
