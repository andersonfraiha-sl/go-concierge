import { describe, expect, it } from 'vitest';

// Cores e medidas só do tema (src/theme). Fora dele, a tela pede cor pelo caminho da paleta
// ('text.secondary') e medida pela escala (TIPOS, PESOS, RAIO, ICONES, GAVETA...).
const REGRAS: readonly [nome: string, padrao: RegExp][] = [
  ['hex', /#[0-9a-fA-F]{3,8}\b/],
  ['rgb/hsl', /\b(rgba?|hsla?)\(/],
  ['tokens.color direto', /\btokens\.color\b/],
  ['px cru', /\b\d+(\.\d+)?px\b/],
  ['fontSize numérico', /\bfontSize:\s*\d/],
  ['fontWeight numérico', /\bfontWeight:\s*\d/],
  ['borderRadius numérico', /\bborderRadius:\s*\d/],
];

// Exceção por arquivo e por regra, sempre com o motivo. Hoje não há nenhuma.
const PERMITIDOS: Record<string, readonly { regra: string; motivo: string }[]> = {};

const telas = Object.entries(
  import.meta.glob<string>('./**/*.{ts,tsx}', { query: '?raw', import: 'default', eager: true }),
).filter(([arquivo]) => !arquivo.startsWith('./theme/') && !/\.test\.tsx?$/.test(arquivo));

const achadosDe = (arquivo: string, conteudo: string): string[] => {
  const liberadas = new Set((PERMITIDOS[arquivo] ?? []).map(({ regra }) => regra));
  return REGRAS.filter(([nome, padrao]) => !liberadas.has(nome) && padrao.test(conteudo)).map(
    ([nome]) => nome,
  );
};

describe('estilo das telas', () => {
  it('encontraAsTelas', () => {
    expect(telas.length).toBeGreaterThan(50);
  });

  it.each(telas)('%s usa só cor e medida do tema', (arquivo, conteudo) => {
    expect(achadosDe(arquivo, conteudo), arquivo).toEqual([]);
  });

  it('exigeMotivoEmToda excecao', () => {
    const semMotivo = Object.values(PERMITIDOS)
      .flat()
      .filter(({ motivo }) => motivo.trim() === '');
    expect(semMotivo).toEqual([]);
  });
});
