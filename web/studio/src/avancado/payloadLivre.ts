import type { Valor } from '@/condominios/contratos';

export const TIPOS = ['texto', 'número', 'sim/não', 'JSON'] as const;
export type Tipo = (typeof TIPOS)[number];

/** `fixo`: o tipo veio gravado ou foi escolhido à mão; só linha nova e sem escolha acompanha o que se digita. */
/** `gravada`: a linha veio do servidor; a chave dela é salva como está, sem aparar. */
export type ParDoPayload = {
  chave: string;
  valor: string;
  tipo: Tipo;
  fixo: boolean;
  gravada?: true;
};

export type Leitura = { ok: true; valor: Valor } | { ok: false; erro: string };

export type ErroDePar = { indice: number; campo: 'chave' | 'valor'; mensagem: string };

const tipoGravado = (valor: Valor): Tipo => {
  if (typeof valor === 'string') return 'texto';
  if (typeof valor === 'number') return 'número';
  if (typeof valor === 'boolean') return 'sim/não';
  return 'JSON';
};

export const payloadEmPares = (payload: Record<string, Valor>): ParDoPayload[] =>
  Object.entries(payload).map(([chave, valor]) => ({
    chave,
    valor: typeof valor === 'string' ? valor : JSON.stringify(valor),
    tipo: tipoGravado(valor),
    fixo: true,
    gravada: true,
  }));

export const parNovo = (): ParDoPayload => ({ chave: '', valor: '', tipo: 'texto', fixo: false });

// Só infere número na forma que volta igual: "01310", "0x1F" e "1e3" ficam texto.
const NUMERO_CANONICO = /^-?(0|[1-9]\d*)(\.\d+)?$/;

export function tipoInferido(texto: string): Tipo {
  if (texto === 'true' || texto === 'false') return 'sim/não';
  return NUMERO_CANONICO.test(texto) ? 'número' : 'texto';
}

const chaveDe = (par: ParDoPayload): string => (par.gravada ? par.chave : par.chave.trim());

export const tipoDe = (texto: string): Tipo => TIPOS.find((t) => t === texto) ?? 'texto';

function lerJson(texto: string): Leitura {
  try {
    return { ok: true, valor: JSON.parse(texto) };
  } catch {
    return { ok: false, erro: 'JSON inválido' };
  }
}

export function lerValor({ valor, tipo }: Pick<ParDoPayload, 'valor' | 'tipo'>): Leitura {
  if (tipo === 'texto') return { ok: true, valor };
  if (tipo === 'JSON') return lerJson(valor);
  if (tipo === 'sim/não') {
    return valor === 'true' || valor === 'false'
      ? { ok: true, valor: valor === 'true' }
      : { ok: false, erro: 'Use true ou false' };
  }
  const numero = Number(valor);
  return valor.trim() !== '' && Number.isFinite(numero)
    ? { ok: true, valor: numero }
    : { ok: false, erro: 'Número inválido' };
}

export const paresEmPayload = (pares: readonly ParDoPayload[]): Record<string, Valor> =>
  Object.fromEntries(
    pares.flatMap((par) => {
      const leitura = lerValor(par);
      const chave = chaveDe(par);
      return chave !== '' && leitura.ok ? [[chave, leitura.valor]] : [];
    }),
  );

/** Valor que não lê no tipo escolhido e chave repetida; linha sem chave é descartada, sem erro. */
export function errosDosPares(pares: readonly ParDoPayload[]): ErroDePar[] {
  const vistas = new Set<string>();
  return pares.flatMap((par, indice) => {
    const chave = chaveDe(par);
    if (chave === '') return [];
    const repetida = vistas.has(chave);
    vistas.add(chave);
    const leitura = lerValor(par);
    return [
      ...(repetida ? [{ indice, campo: 'chave' as const, mensagem: 'Chave repetida' }] : []),
      ...(leitura.ok ? [] : [{ indice, campo: 'valor' as const, mensagem: leitura.erro }]),
    ];
  });
}
