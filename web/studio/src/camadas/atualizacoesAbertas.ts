import type { Atualizacoes, Camada } from './contratos';

type Resumo = Atualizacoes['atualizacoes'][number];
type Aviso = Atualizacoes['avisos'][number];

const mesmaCamada = (a: Camada, b: Camada): boolean =>
  a.skill === b.skill && a.tipo === b.tipo && a.id === b.id;

// Recusada e parcial seguem disponíveis para revisar, mas não puxam a atenção de novo.
const estaPedindoDecisao = (atualizacao: Resumo): boolean => atualizacao.estado === 'pendente';

export function atualizacoesDaCamada(dados: Atualizacoes, camada: Camada): Resumo[] {
  return dados.atualizacoes.filter(
    (atualizacao) => estaPedindoDecisao(atualizacao) && mesmaCamada(atualizacao.camada, camada),
  );
}

export function avisosDaCamada(dados: Atualizacoes, camada: Camada): Aviso[] {
  return dados.avisos.filter((aviso) => mesmaCamada(aviso.camada, camada));
}

/** Skills que têm atualização esperando decisão em alguma camada que a pessoa enxerga. */
export function skillsComAtualizacao(dados: Atualizacoes): ReadonlySet<string> {
  return new Set(
    dados.atualizacoes.filter(estaPedindoDecisao).map((atualizacao) => atualizacao.camada.skill),
  );
}

const TEXTOS_DA_BASE: Record<string, string> = {
  titulo: 'o título',
  resumo: 'o resumo',
  quandoUsar: 'o texto de quando usar',
  frasesDeExemplo: 'as frases de exemplo',
  palavrasChave: 'as palavras-chave',
};

function juntar(itens: readonly string[]): string {
  if (itens.length <= 1) return itens.join('');
  return `${itens.slice(0, -1).join(', ')} e ${itens[itens.length - 1]}`;
}

function descreverEtapas(quantidade: number): string {
  return quantidade === 1 ? 'uma etapa do passo a passo' : `${quantidade} etapas do passo a passo`;
}

function descreverUm(trecho: string): string {
  const [tipo, nome = ''] = trecho.split(/:(.*)/s);
  if (tipo === 'texto') return TEXTOS_DA_BASE[nome] ?? trecho;
  if (tipo === 'orientacao') return 'a orientação';
  if (tipo === 'informacao') return `a informação "${nome}"`;
  if (trecho === 'fluxo:ordem') return 'a ordem das etapas';
  return trecho;
}

/** "o título, a orientação e 2 etapas do passo a passo": os ids dos trechos em palavras de gente. */
export function descreverTrechos(trechos: readonly string[]): string {
  const etapas = trechos.filter((trecho) => trecho.startsWith('etapa:')).length;
  const demais = trechos.filter((trecho) => !trecho.startsWith('etapa:')).map(descreverUm);
  return juntar(etapas > 0 ? [...demais, descreverEtapas(etapas)] : demais);
}
