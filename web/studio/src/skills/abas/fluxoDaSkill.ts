import { z } from 'zod';
import { useFerramentas } from '@/catalogo/contratos';
import { type Bloco, type ProblemaFluxo, ProblemaFluxoSchema } from '@/fluxo/modelo/tipos';
import type { CatalogoDeFerramentas } from '@/fluxo/painel/camposPorTipo';
import { ApiError } from '@/shared/api/http';
import { listaDoGo } from '@/shared/api/listaDoGo';
import { tokens } from '@/theme/tokens';

const RecusaSchema = z.object({ problemas: listaDoGo(ProblemaFluxoSchema) });

/** Problemas que o servidor mandou junto do 422; null para qualquer outra falha. */
export function problemasDaRecusa(erro: unknown): ProblemaFluxo[] | null {
  if (!(erro instanceof ApiError) || erro.status !== 422) return null;
  const lida = RecusaSchema.safeParse(erro.corpo);
  return lida.success ? lida.data.problemas : null;
}

export const chaveDoProblema = (p: ProblemaFluxo): string =>
  `${p.blocoId ?? ''}|${p.codigo}|${p.mensagem}`;

/** Os locais valem sempre; os do servidor, os do fluxo todo e os de blocos que ainda existem, sem repetir os locais. */
export function juntarProblemas(
  locais: ProblemaFluxo[],
  doServidor: ProblemaFluxo[],
  blocos: readonly Bloco[],
): ProblemaFluxo[] {
  const ids = new Set(blocos.map((b) => b.id));
  const jaVistos = new Set(locais.map(chaveDoProblema));
  const aindaValidos = doServidor.filter(
    (p) => (p.blocoId === undefined || ids.has(p.blocoId)) && !jaVistos.has(chaveDoProblema(p)),
  );
  return [...locais, ...aindaValidos];
}

/** Primeiro `bN` livre: curto, legível e sem colidir com os IDs que vieram do servidor. */
export function novoId(blocos: readonly Bloco[]): string {
  const usados = new Set(blocos.map((b) => b.id));
  let numero = blocos.length + 1;
  while (usados.has(`b${numero}`)) numero += 1;
  return `b${numero}`;
}

export function useCatalogoDeFerramentas(): CatalogoDeFerramentas {
  const consulta = useFerramentas();
  if (consulta.isPending) return { estado: 'carregando' };
  if (consulta.isError) return { estado: 'erro' };
  return { estado: 'pronto', ferramentas: consulta.data.recursos };
}

export const corDoNivel = (nivel: ProblemaFluxo['nivel']): string =>
  nivel === 'erro' ? tokens.color.studio.issueError : tokens.color.studio.issueWarning;

/** Ids dos blocos desta skill que uma proposta do Copiloto acabou de mudar. */
export function blocosRealcados(alvos: ReadonlySet<string>, nome: string): ReadonlySet<string> {
  const prefixo = `skill:${nome}:bloco:`;
  return new Set(
    [...alvos].filter((alvo) => alvo.startsWith(prefixo)).map((alvo) => alvo.slice(prefixo.length)),
  );
}
