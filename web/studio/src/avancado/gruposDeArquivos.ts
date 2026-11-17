import type { Arquivo } from '@/catalogo/contratos';

export type NomeDoGrupo = 'Raiz' | 'Conhecimento' | 'Rotas' | 'Modelos e testes';

/** `exibicao`: nome sem o prefixo da pasta quando ele é redundante (ex. "routers/rotas.md" → "rotas.md"); `caminho` continua completo para abrir/salvar/excluir. */
export type ArquivoNaArvore = Arquivo & { exibicao: string };

export type GrupoDeArquivos = { grupo: NomeDoGrupo; arquivos: ArquivoNaArvore[] };

function caminhoSemPrefixoDaPasta(pasta: string, caminho: string): string {
  const prefixo = `${pasta}/`;
  return caminho.startsWith(prefixo) ? caminho.slice(prefixo.length) : caminho;
}

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
  const porGrupo = new Map<NomeDoGrupo, ArquivoNaArvore[]>();
  for (const [pasta, lista] of Object.entries(arquivos)) {
    const grupo = GRUPO_DA_PASTA[pasta];
    if (!grupo || lista.length === 0) continue;
    const comExibicao = lista.map((arquivo) => ({
      ...arquivo,
      exibicao: caminhoSemPrefixoDaPasta(pasta, arquivo.caminho),
    }));
    porGrupo.set(grupo, [...(porGrupo.get(grupo) ?? []), ...comExibicao]);
  }
  return ORDEM_DOS_GRUPOS.flatMap((grupo) => {
    const arquivosDoGrupo = porGrupo.get(grupo);
    return arquivosDoGrupo ? [{ grupo, arquivos: arquivosDoGrupo }] : [];
  });
}
