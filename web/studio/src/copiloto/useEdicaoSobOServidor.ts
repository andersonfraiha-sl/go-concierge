import { useEffect, useRef, useState } from 'react';
import { useCopiloto } from './CopilotoProvider';

/** Quem mudou o servidor sob a edição: o texto do aviso e do botão dependem disso. */
export type OrigemDoConflito = 'copiloto' | 'outra';

/** `V` é a versão da checagem otimista (`string`) ou `null` em quem grava sem versão. */
type Opcoes<T, V extends string | null> = {
  /** O que a consulta traz agora do servidor. */
  servidor: T;
  /** Ids de realce que uma proposta do Copiloto acende ao mudar este formulário. */
  alvos: readonly string[];
  versao: V;
  /** O que o formulário mostra agora. */
  local: T;
  /** Troca o que o formulário mostra pelo valor do servidor. */
  resetar: (valor: T) => void;
  /** Grava sobre a versão em que a edição se baseou; devolve a versão gravada. */
  salvar: (valor: T, versao: V) => Promise<V>;
  /** Forma do valor no servidor (ex.: sem chips vazios), para comparar sem falso conflito. */
  normalizar?: (valor: T) => T;
};

type EdicaoSobOServidor<T> = {
  /** `salvar` que não regrava o que já está no servidor; para o salvamento automático. */
  salvar: (valor: T) => Promise<void>;
  /** O servidor mudou enquanto havia edição não salva: nada grava até a pessoa escolher. */
  conflito: OrigemDoConflito | null;
  /** Para `useAutosave({ bloqueio })` enquanto há conflito. */
  bloqueio: string | null;
  /** Grava a edição da pessoa por cima da versão nova (agora sabendo qual é). */
  manterOMeu: () => void;
  /** Descarta a edição e mostra a versão nova. */
  usarONovo: () => void;
};

type Gravado<T, V> = { valor: T; versao: V };

const MOTIVO_DO_BLOQUEIO = 'escolha qual versão fica';
function iguais(esquerda: unknown, direita: unknown): boolean {
  return JSON.stringify(esquerda) === JSON.stringify(direita);
}

/**
 * Formulário aberto quando o servidor muda por fora (proposta do Copiloto, colega editando):
 * sem edição pendente, passa a mostrar a versão nova; com edição pendente, nada é gravado nem
 * trocado até a pessoa escolher. Uma leitura mais velha que a última gravação é ignorada.
 */
export function useEdicaoSobOServidor<T, V extends string | null>({
  servidor,
  alvos,
  versao,
  local,
  resetar,
  salvar,
  normalizar = (valor) => valor,
}: Opcoes<T, V>): EdicaoSobOServidor<T> {
  const { alvosRealcados } = useCopiloto();
  const doCopiloto = alvos.some((alvo) => alvosRealcados.has(alvo));
  // A base da edição: o que o formulário sabe estar gravado, e em que versão.
  const base = useRef<Gravado<T, V>>({ valor: servidor, versao });
  const doServidor = useRef<Gravado<T, V>>({ valor: servidor, versao });
  const superadas = useRef(new Set<string>());
  const localAtual = useRef(local);
  const realcado = useRef(doCopiloto);
  const [conflito, setConflito] = useState<OrigemDoConflito | null>(null);
  const igualNoServidor = (esquerda: T, direita: T) =>
    iguais(normalizar(esquerda), normalizar(direita));

  useEffect(() => {
    localAtual.current = local;
    realcado.current = doCopiloto;
  });

  // O realce pode chegar depois da leitura nova: o conflito passa a ser do Copiloto.
  useEffect(() => {
    if (doCopiloto) setConflito((atual) => (atual === null ? null : 'copiloto'));
  }, [doCopiloto]);

  const adotar = (novo: Gravado<T, V>) => {
    const anterior = base.current.versao;
    if (typeof anterior === 'string' && anterior !== novo.versao) superadas.current.add(anterior);
    base.current = novo;
  };

  // biome-ignore lint/correctness/useExhaustiveDependencies: reage só ao que chega do servidor; o resto vem por ref.
  useEffect(() => {
    const chegou = { valor: servidor, versao };
    doServidor.current = chegou;
    if (typeof versao === 'string' && superadas.current.has(versao)) return;
    if (igualNoServidor(servidor, base.current.valor)) return adotar(chegou);
    if (!igualNoServidor(localAtual.current, base.current.valor)) {
      setConflito(realcado.current ? 'copiloto' : 'outra');
      return;
    }
    adotar(chegou);
    resetar(servidor);
  }, [servidor, versao]);

  const salvarSoOQueMudou = async (valor: T): Promise<void> => {
    if (igualNoServidor(valor, base.current.valor)) return;
    // Marca antes: a resposta pode atualizar o cache (e o `servidor`) antes do await voltar.
    const anterior = base.current;
    base.current = { valor, versao: anterior.versao };
    try {
      adotar({ valor, versao: await salvar(valor, anterior.versao) });
    } catch (erro) {
      base.current = anterior;
      throw erro;
    }
  };

  return {
    salvar: salvarSoOQueMudou,
    conflito,
    bloqueio: conflito === null ? null : MOTIVO_DO_BLOQUEIO,
    manterOMeu: () => {
      adotar(doServidor.current);
      setConflito(null);
    },
    usarONovo: () => {
      adotar(doServidor.current);
      resetar(doServidor.current.valor);
      setConflito(null);
    },
  };
}
