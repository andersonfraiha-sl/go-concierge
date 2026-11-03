import type { SxProps, Theme } from '@mui/material';
import { useQueryClient } from '@tanstack/react-query';
import { createContext, type ReactNode, useContext, useEffect, useRef, useState } from 'react';
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
  status: Status;
  mensagens: Mensagem[];
  enviar: (texto: string) => Promise<void>;
  aplicar: (proposta: Proposta) => Promise<void>;
  descartar: (idDaProposta: string) => void;
  definirContexto: (contexto: ContextoDoCopiloto) => void;
  alvosRealcados: ReadonlySet<string>;
  realcar: (alvos: readonly string[]) => void;
};

const CHAVE_ABERTO = 'studio.copilotoAberto';
const NENHUM_ALVO: ReadonlySet<string> = new Set();
const DURACAO_DO_REALCE_MS = 1500;
const CONTEXTO_INICIAL: ContextoDoCopiloto = { tela: 'skills' };

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
  const [alvosRealcados, setAlvosRealcados] = useState<ReadonlySet<string>>(NENHUM_ALVO);
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
      acrescentar({ papel: 'copiloto', texto: mensagemDe(erro), propostas: [], tom: 'erro' });
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

  const realcar = (alvos: readonly string[]) => {
    setAlvosRealcados((atuais) => new Set([...atuais, ...alvos]));
    setTimeout(() => {
      setAlvosRealcados((atuais) => new Set([...atuais].filter((alvo) => !alvos.includes(alvo))));
    }, DURACAO_DO_REALCE_MS);
  };

  const valor: ValorDoCopiloto = {
    aberto,
    alternar,
    status: statusDe(estado.data?.disponivel, pensando),
    mensagens,
    enviar,
    aplicar: aindaNaoAplica,
    descartar,
    definirContexto: (proximo) => {
      contexto.current = proximo;
    },
    alvosRealcados,
    realcar,
  };

  return <CopilotoContext.Provider value={valor}>{children}</CopilotoContext.Provider>;
}

// A Task 26 troca por aplicar de verdade, chamando `realcar` com os alvos mudados.
function aindaNaoAplica(_proposta: Proposta): Promise<void> {
  return Promise.reject(new Error('O Copiloto ainda não aplica propostas.'));
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
