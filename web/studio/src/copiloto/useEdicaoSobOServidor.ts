import { useEffect, useRef, useState } from 'react';

type Opcoes<T> = {
  /** O que a consulta traz agora do servidor. */
  servidor: T;
  /** O que o formulário mostra agora. */
  local: T;
  /** Troca o que o formulário mostra pelo valor do servidor. */
  resetar: (valor: T) => void;
  salvar: (valor: T) => Promise<void>;
};

type EdicaoSobOServidor<T> = {
  /** `salvar` que lembra o que já está no servidor e não regrava o mesmo valor. */
  salvar: (valor: T) => Promise<void>;
  /** O servidor mudou (ex.: proposta do Copiloto aplicada) enquanto havia edição não salva. */
  mudouSobEdicao: boolean;
  dispensarAviso: () => void;
};

const iguais = (a: unknown, b: unknown): boolean => JSON.stringify(a) === JSON.stringify(b);

/**
 * Formulário aberto quando o valor do servidor muda por fora: sem edição pendente, o formulário
 * passa a mostrar o novo valor; com edição pendente, a edição fica e `mudouSobEdicao` liga o aviso.
 * Nunca sobrescreve um dos lados em silêncio.
 */
export function useEdicaoSobOServidor<T>({
  servidor,
  local,
  resetar,
  salvar,
}: Opcoes<T>): EdicaoSobOServidor<T> {
  // O último valor que o formulário sabe estar gravado: o lido ou o que ele mesmo salvou.
  const gravado = useRef(servidor);
  const localAtual = useRef(local);
  const [mudouSobEdicao, setMudouSobEdicao] = useState(false);

  useEffect(() => {
    localAtual.current = local;
  });

  useEffect(() => {
    if (iguais(servidor, gravado.current)) return;
    const editando = !iguais(localAtual.current, gravado.current);
    gravado.current = servidor;
    if (iguais(localAtual.current, servidor)) return;
    if (editando) setMudouSobEdicao(true);
    else resetar(servidor);
  }, [servidor, resetar]);

  const salvarSoOQueMudou = async (valor: T): Promise<void> => {
    if (iguais(valor, gravado.current)) return;
    // Marca antes: a resposta pode atualizar o cache (e o `servidor`) antes do await voltar.
    const anterior = gravado.current;
    gravado.current = valor;
    try {
      await salvar(valor);
    } catch (erro) {
      gravado.current = anterior;
      throw erro;
    }
  };

  return {
    salvar: salvarSoOQueMudou,
    mudouSobEdicao,
    dispensarAviso: () => setMudouSobEdicao(false),
  };
}
