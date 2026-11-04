import type { Arquivo } from '@/catalogo/contratos';

export type NomeDoGrupo = 'Raiz' | 'Conhecimento' | 'Rotas' | 'Modelos e testes';

export type GrupoDeArquivos = { grupo: NomeDoGrupo; arquivos: Arquivo[] };

/** A ordem fixa em que os grupos aparecem na árvore do Avançado. */
const ORDEM_DOS_GRUPOS: readonly NomeDoGrupo[] = [
  'Raiz',
  'Conhecimento',
  'Rotas',
  'Modelos e testes',
];

const GRUPO_DA_PASTA: Record<string, NomeDoGrupo> = {
  'agent.yaml': 'Raiz',
  'persona.md': 'Raiz',
  'caso.yaml': 'Raiz',
  rules: 'Raiz',
  workflows: 'Raiz',
  knowledge: 'Conhecimento',
  routers: 'Rotas',
  assets: 'Modelos e testes',
  evals: 'Modelos e testes',
};

/** Agrupa os arquivos de um item nas quatro seções da árvore do Avançado; pastas sem arquivo não aparecem. */
export function agruparArquivos(arquivos: Record<string, Arquivo[]>): GrupoDeArquivos[] {
  const porGrupo = new Map<NomeDoGrupo, Arquivo[]>();
  for (const [pasta, lista] of Object.entries(arquivos)) {
    const grupo = GRUPO_DA_PASTA[pasta];
    if (!grupo || lista.length === 0) continue;
    porGrupo.set(grupo, [...(porGrupo.get(grupo) ?? []), ...lista]);
  }
  return ORDEM_DOS_GRUPOS.flatMap((grupo) => {
    const arquivosDoGrupo = porGrupo.get(grupo);
    return arquivosDoGrupo ? [{ grupo, arquivos: arquivosDoGrupo }] : [];
  });
}
