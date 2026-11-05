export type SecaoDeMarkdown = { titulo: string; corpo: string };

const TITULO = /^## (.+)$/gm;

/**
 * Divide um Markdown pelos títulos `## `: cada um vira uma regra (um cartão). Sem nenhum `## `,
 * devolve uma lista vazia (quem chama cai no cartão único de sempre, pelo nome do arquivo).
 * O corpo de cada seção guarda tudo até o próximo título, byte a byte: juntarSecoes desfaz
 * exatamente a divisão quando nenhuma seção muda.
 */
export function dividirEmSecoes(conteudo: string): SecaoDeMarkdown[] {
  const titulos = [...conteudo.matchAll(TITULO)].map((m) => ({
    titulo: (m[1] ?? '').trim(),
    fimDoTitulo: (m.index ?? 0) + m[0].length,
    inicio: m.index ?? 0,
  }));
  return titulos.map((atual, i) => {
    const fim = titulos[i + 1]?.inicio ?? conteudo.length;
    return {
      titulo: atual.titulo,
      corpo: conteudo.slice(atual.fimDoTitulo, fim).replace(/^\n/, ''),
    };
  });
}

/** Desfaz dividirEmSecoes: reconstrói o Markdown de um arquivo a partir das seções atuais. */
export function juntarSecoes(secoes: readonly SecaoDeMarkdown[]): string {
  return secoes.map(({ titulo, corpo }) => `## ${titulo}\n${corpo}`).join('');
}
