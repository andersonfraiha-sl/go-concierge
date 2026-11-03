import type { Bloco, Fluxo } from './tipos';

export const ALTURA_BLOCO = 72;
export const ESPACO = 48;
export const LARGURA_BLOCO = 280;

const clamp = (valor: number, minimo: number, maximo: number): number =>
  Math.max(minimo, Math.min(valor, maximo));

const comBlocos = (f: Fluxo, blocos: Bloco[]): Fluxo => ({ ...f, blocos });

/** Quantos blocos podem ser reordenados: todos, menos o Responder que fecha o fluxo. */
function quantosSeMovem(f: Fluxo): number {
  return f.blocos.at(-1)?.tipo === 'responder' ? f.blocos.length - 1 : f.blocos.length;
}

export function inserirBloco(f: Fluxo, indice: number, bloco: Bloco): Fluxo {
  const posicao = clamp(indice, 0, quantosSeMovem(f));
  return comBlocos(f, [...f.blocos.slice(0, posicao), bloco, ...f.blocos.slice(posicao)]);
}

export function moverBloco(f: Fluxo, de: number, para: number): Fluxo {
  const movel = quantosSeMovem(f);
  const bloco = de >= 0 && de < movel ? f.blocos[de] : undefined;
  if (!bloco) return f;
  const semOBloco = f.blocos.filter((_, i) => i !== de);
  const posicao = clamp(para, 0, movel - 1);
  return comBlocos(f, [...semOBloco.slice(0, posicao), bloco, ...semOBloco.slice(posicao)]);
}

export function removerBloco(f: Fluxo, id: string): Fluxo {
  const alvo = f.blocos.find((b) => b.id === id);
  if (!alvo || alvo.tipo === 'responder') return f;
  return comBlocos(f, f.blocos.filter((b) => b.id !== id));
}

export function atualizarBloco(f: Fluxo, bloco: Bloco): Fluxo {
  return comBlocos(f, f.blocos.map((b) => (b.id === bloco.id ? bloco : b)));
}

/** Posição em que um bloco arrastado cai, dado o y do soltar no canvas. */
export function indiceAoSoltar(y: number, total: number): number {
  return clamp(Math.round(y / (ALTURA_BLOCO + ESPACO)), 0, total - 2);
}
