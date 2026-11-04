/** Rótulo com parêntese curto no fim: separa a unidade pra virar adornment em vez de texto no label. */
export function separarUnidade(rotulo: string): { rotulo: string; unidade: string | null } {
  const casado = /^(.*) \(([^()]{1,12})\)$/.exec(rotulo);
  if (casado === null) return { rotulo, unidade: null };
  const [, base, unidade] = casado;
  if (base === undefined || unidade === undefined) return { rotulo, unidade: null };
  return { rotulo: base, unidade };
}
