import { type QueryClient, type QueryKey, useQueryClient } from '@tanstack/react-query';
import { useRef, useState } from 'react';
import { buscarEstado, chaveDoEstado } from '@/condominios/contratos';
import {
  type Ambiente,
  aplicarOperacao,
  DESISTIU,
  operacaoDaVez,
  TextosNaoSalvaram,
} from './aplicar';
import type { Proposta } from './contratos';

/** O que só o cartão sabe: quem aplica (escopo) e como perguntar antes de excluir. */
export type PedidoDeAplicacao = Pick<Ambiente, 'escopo' | 'confirmar'>;

export type AplicacaoDePropostas = {
  /** Grava pelas mesmas rotas das telas; falha rejeita e não muda nada aqui. */
  aplicar: (proposta: Proposta, pedido: PedidoDeAplicacao) => Promise<void>;
  aplicadas: ReadonlySet<string>;
  /** Id da proposta sendo aplicada; as outras esperam. */
  emCurso: string | null;
};

export const MENSAGEM_DE_ESPERA = 'Aguarde a anterior';
const CHAVE_DAS_SKILLS: QueryKey = ['skills'];

type Contexto = {
  cliente: QueryClient;
  /** Skill que a proposta já criou: tentar de novo só reenvia os textos. */
  criadas: Map<string, string>;
  realcar: (alvos: readonly string[]) => void;
  marcarAplicada: (id: string) => void;
};

const invalidar = (cliente: QueryClient, chaves: readonly QueryKey[]) =>
  Promise.all(chaves.map((queryKey) => cliente.invalidateQueries({ queryKey })));

async function aplicarUma(
  { cliente, criadas, realcar, marcarAplicada }: Contexto,
  proposta: Proposta,
  { escopo, confirmar }: PedidoDeAplicacao,
): Promise<void> {
  const lerEstado = () =>
    cliente.fetchQuery({
      queryKey: chaveDoEstado(escopo),
      queryFn: () => buscarEstado(escopo),
      staleTime: 0,
    });
  const operacao = operacaoDaVez(proposta.operacao, criadas.get(proposta.id));
  try {
    const resultado = await aplicarOperacao(operacao, { escopo, confirmar, lerEstado });
    if (resultado === DESISTIU) return;
    await invalidar(cliente, resultado.invalidar);
    realcar(resultado.alvos);
    marcarAplicada(proposta.id);
  } catch (erro) {
    await lembrarSkillCriada(cliente, criadas, proposta.id, erro);
    throw erro;
  }
}

async function lembrarSkillCriada(
  cliente: QueryClient,
  criadas: Map<string, string>,
  idDaProposta: string,
  erro: unknown,
): Promise<void> {
  if (!(erro instanceof TextosNaoSalvaram)) return;
  criadas.set(idDaProposta, erro.nome);
  await invalidar(cliente, [CHAVE_DAS_SKILLS]);
}

/**
 * Uma aplicação por vez: ajustes e orientação trocam o mapa inteiro do condomínio, e a segunda
 * precisa ler o estado que a primeira gravou, ou apaga a primeira.
 */
export function useAplicarPropostas(realcar: (alvos: readonly string[]) => void) {
  const cliente = useQueryClient();
  const [aplicadas, setAplicadas] = useState<ReadonlySet<string>>(() => new Set());
  const [emCurso, setEmCurso] = useState<string | null>(null);
  const ocupado = useRef(false);
  const criadas = useRef(new Map<string, string>());
  const marcarAplicada = (id: string) => setAplicadas((atuais) => new Set([...atuais, id]));

  const aplicar = async (proposta: Proposta, pedido: PedidoDeAplicacao) => {
    if (ocupado.current) throw new Error(MENSAGEM_DE_ESPERA);
    ocupado.current = true;
    setEmCurso(proposta.id);
    try {
      await aplicarUma(
        { cliente, criadas: criadas.current, realcar, marcarAplicada },
        proposta,
        pedido,
      );
    } finally {
      ocupado.current = false;
      setEmCurso(null);
    }
  };

  return { aplicar, aplicadas, emCurso } satisfies AplicacaoDePropostas;
}
