import { type QueryKey, useQueryClient } from '@tanstack/react-query';
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

/**
 * Uma aplicação por vez: ajustes e orientação trocam o mapa inteiro do condomínio, e a segunda
 * precisa ler o estado que a primeira gravou, ou apaga a primeira.
 */
export function useAplicarPropostas(realcar: (alvos: readonly string[]) => void) {
  const cliente = useQueryClient();
  const [aplicadas, setAplicadas] = useState<ReadonlySet<string>>(() => new Set());
  const [emCurso, setEmCurso] = useState<string | null>(null);
  const ocupado = useRef(false);
  // Skill que a proposta já criou: tentar de novo só reenvia os textos.
  const criadas = useRef(new Map<string, string>());

  const invalidar = (chaves: readonly QueryKey[]) =>
    Promise.all(chaves.map((queryKey) => cliente.invalidateQueries({ queryKey })));

  const executar = async (proposta: Proposta, { escopo, confirmar }: PedidoDeAplicacao) => {
    const lerEstado = () =>
      cliente.fetchQuery({
        queryKey: chaveDoEstado(escopo),
        queryFn: () => buscarEstado(escopo),
        staleTime: 0,
      });
    const operacao = operacaoDaVez(proposta.operacao, criadas.current.get(proposta.id));
    try {
      const resultado = await aplicarOperacao(operacao, { escopo, confirmar, lerEstado });
      if (resultado === DESISTIU) return;
      await invalidar(resultado.invalidar);
      realcar(resultado.alvos);
      setAplicadas((atuais) => new Set([...atuais, proposta.id]));
    } catch (erro) {
      if (erro instanceof TextosNaoSalvaram) {
        criadas.current.set(proposta.id, erro.nome);
        await invalidar([CHAVE_DAS_SKILLS]);
      }
      throw erro;
    }
  };

  const aplicar = async (proposta: Proposta, pedido: PedidoDeAplicacao) => {
    if (ocupado.current) throw new Error(MENSAGEM_DE_ESPERA);
    ocupado.current = true;
    setEmCurso(proposta.id);
    try {
      await executar(proposta, pedido);
    } finally {
      ocupado.current = false;
      setEmCurso(null);
    }
  };

  return { aplicar, aplicadas, emCurso } satisfies AplicacaoDePropostas;
}
