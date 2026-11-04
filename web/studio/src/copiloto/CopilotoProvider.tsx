import type { SxProps, Theme } from '@mui/material';
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

const CHAVE_ABERTO = 'studio.copilotoAberto';
const NENHUM_ALVO: ReadonlySet<string> = new Set();
const DURACAO_DO_REALCE_MS = 1500;
const CONTEXTO_INICIAL: ContextoDoCopiloto = { tela: 'skills' };
const MENSAGEM_DE_DEMORA = 'O Copiloto demorou demais.';

const CopilotoContext = createContext<ValorDoCopiloto | null>(null);

export function useCopiloto(): ValorDoCopiloto {
  const valor = useContext(CopilotoContext);
  if (!valor) throw new Error('useCopiloto fora do CopilotoProvider');
  return valor;
}

/** Fundo de realce por 1500 ms no campo que uma proposta do Copiloto acabou de mudar. */
export function useRealceDoCopiloto(alvo: string): SxProps<Theme> {
  const { alvosRealcados } = useCopiloto();
  const transicao = { transition: 'background-color 300ms ease-out' };
  if (!alvosRealcados.has(alvo)) return transicao;
  return { ...transicao, bgcolor: tokens.color.realceDoCopiloto };
}

export function CopilotoProvider({ children }: { children: ReactNode }) {
  const [aberto, setAberto] = useState(lerAberto);
  const [mensagens, setMensagens] = useState<Mensagem[]>([]);
  const [pensando, setPensando] = useState(false);
  const [alvosRealcados, realcar] = useAlvosRealcados();
  const aplicacao = useAplicarPropostas(realcar);
  const botaoDoCopiloto = useRef<HTMLButtonElement>(null);
  // Ref para o pedido em curso (sempre a última tela, mesmo meio a um envio); state para quem
  // lê no render (ex.: sugestões do estado vazio) — só muda, e só então redesenha, quando a
  // tela de fato troca, nunca a cada chamada de definirContexto com o mesmo valor.
  const contextoDoPedido = useRef(CONTEXTO_INICIAL);
  const [contexto, setContexto] = useState<ContextoDoCopiloto>(CONTEXTO_INICIAL);
  const pedidoEmCurso = useRef<AbortController | null>(null);
  const estado = useEstadoDoCopiloto(aberto);
  const queryClient = useQueryClient();

  useEffect(() => () => pedidoEmCurso.current?.abort(), []);

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

  // O id da proposta vem do servidor como índice ("0", "1"…) e se repete entre respostas.
  const acrescentar = (mensagem: Omit<Mensagem, 'id'>) => {
    const id = crypto.randomUUID();
    const propostas = mensagem.propostas.map((proposta) => ({
      ...proposta,
      id: `${id}:${proposta.id}`,
    }));
    setMensagens((atuais) => [...atuais, { ...mensagem, id, propostas }]);
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
      if (controle.signal.aborted) return;
      if (erro instanceof ApiError && erro.codigo === 'COPILOTO_INDISPONIVEL') {
        await queryClient.invalidateQueries({ queryKey: CHAVE_DO_ESTADO });
      }
      acrescentar({ papel: 'copiloto', texto: textoDoErro(erro), propostas: [], tom: 'erro' });
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
    definirContexto: (proximo) => {
      contextoDoPedido.current = proximo;
      setContexto((atual) => (contextosIguais(atual, proximo) ? atual : proximo));
    },
    alvosRealcados,
    realcar,
  };

  return <CopilotoContext.Provider value={valor}>{children}</CopilotoContext.Provider>;
}

/** Um prazo por alvo: realçar de novo reinicia os 1500 ms daquele alvo; sair da tela limpa todos. */
function useAlvosRealcados(): [ReadonlySet<string>, (alvos: readonly string[]) => void] {
  const [alvosRealcados, setAlvosRealcados] = useState<ReadonlySet<string>>(NENHUM_ALVO);
  const prazos = useRef(new Map<string, ReturnType<typeof setTimeout>>());

  useEffect(() => {
    const prazosAtivos = prazos.current;
    return () => {
      for (const prazo of prazosAtivos.values()) clearTimeout(prazo);
      prazosAtivos.clear();
    };
  }, []);

  const apagar = (alvo: string) => {
    prazos.current.delete(alvo);
    setAlvosRealcados((atuais) => new Set([...atuais].filter((atual) => atual !== alvo)));
  };

  const realcar = (alvos: readonly string[]) => {
    setAlvosRealcados((atuais) => new Set([...atuais, ...alvos]));
    for (const alvo of alvos) {
      clearTimeout(prazos.current.get(alvo));
      prazos.current.set(
        alvo,
        setTimeout(() => apagar(alvo), DURACAO_DO_REALCE_MS),
      );
    }
  };

  return [alvosRealcados, realcar];
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

function lerAberto(): boolean {
  try {
    return localStorage.getItem(CHAVE_ABERTO) === 'true';
  } catch (cause) {
    console.warn('ler preferência do Copiloto:', cause);
    return false;
  }
}

function gravarAberto(aberto: boolean): void {
  try {
    localStorage.setItem(CHAVE_ABERTO, String(aberto));
  } catch (cause) {
    console.warn('gravar preferência do Copiloto:', cause);
  }
}
