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
import type { Estado } from '@/condominios/contratos';
import { ApiError } from '@/shared/api/http';
import { mensagemDe } from '@/shared/api/mensagemDe';
import { tokens } from '@/theme/tokens';
import { type Ambiente, aplicarOperacao, DESISTIU } from './aplicar';
import {
  CHAVE_DO_ESTADO,
  type ContextoDoCopiloto,
  conversar,
  historicoDe,
  type Proposta,
  useEstadoDoCopiloto,
} from './contratos';

export type Status = 'pronto' | 'pensando' | 'indisponivel';

export type Mensagem = {
  id: string;
  papel: 'pessoa' | 'copiloto';
  texto: string;
  propostas: Proposta[];
  tom: 'normal' | 'erro';
};

type ValorDoCopiloto = {
  aberto: boolean;
  alternar: () => void;
  /** Fecha pelo X do painel e devolve o foco ao botão do header, que continua na tela. */
  fechar: () => void;
  botaoDoCopiloto: RefObject<HTMLButtonElement | null>;
  status: Status;
  mensagens: Mensagem[];
  enviar: (texto: string) => Promise<void>;
  /** Grava pelas mesmas rotas das telas; falha rejeita e não muda nada aqui. */
  aplicar: (proposta: Proposta, pedido: PedidoDeAplicacao) => Promise<void>;
  aplicadas: ReadonlySet<string>;
  descartar: (idDaProposta: string) => void;
  definirContexto: (contexto: ContextoDoCopiloto) => void;
  alvosRealcados: ReadonlySet<string>;
  realcar: (alvos: readonly string[]) => void;
};

/** O que só o cartão sabe: quem aplica (escopo) e como perguntar antes de excluir. */
export type PedidoDeAplicacao = Pick<Ambiente, 'escopo' | 'confirmar'>;

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
  const [aplicadas, setAplicadas] = useState<ReadonlySet<string>>(() => new Set());
  const [alvosRealcados, realcar] = useAlvosRealcados();
  const botaoDoCopiloto = useRef<HTMLButtonElement>(null);
  // Só leitura na hora de enviar, nunca no render: troca de tela não redesenha o painel.
  const contexto = useRef(CONTEXTO_INICIAL);
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
      contexto: contexto.current,
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

  const aplicar = async (proposta: Proposta, { escopo, confirmar }: PedidoDeAplicacao) => {
    const lerEstado = () => queryClient.getQueryData<Estado>(['estado-condominios', escopo]);
    const resultado = await aplicarOperacao(proposta.operacao, { escopo, confirmar, lerEstado });
    if (resultado === DESISTIU) return;
    await Promise.all(
      resultado.invalidar.map((queryKey) => queryClient.invalidateQueries({ queryKey })),
    );
    realcar(resultado.alvos);
    setAplicadas((atuais) => new Set([...atuais, proposta.id]));
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
    enviar,
    aplicar,
    aplicadas,
    descartar,
    definirContexto: (proximo) => {
      contexto.current = proximo;
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
