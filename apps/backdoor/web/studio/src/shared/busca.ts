/** NFD + remoção de marca diacrítica + minúsculas + trim: compara texto ignorando acento e caixa. */
export function normalizarBusca(texto: string): string {
  return texto.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase().trim();
}

/** Busca vazia sempre acha; senão, basta um alvo conter o termo normalizado. */
export function contemBusca(alvos: readonly string[], busca: string): boolean {
  const termo = normalizarBusca(busca);
  if (termo === '') return true;
  return alvos.some((alvo) => normalizarBusca(alvo).includes(termo));
}
