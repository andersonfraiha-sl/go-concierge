import { describe, expect, it } from 'vitest';
import { dividirEmSecoes, juntarSecoes } from './secoesDoMarkdown';

describe('dividirEmSecoes', () => {
  it('semTituloNenhumNaoDivide', () => {
    expect(dividirEmSecoes('Seja gentil.')).toEqual([]);
  });

  it('divideCadaTituloEmUmaSecao', () => {
    const conteudo = '## Horário\nSó liga de dia.\n\n## Tom\nSeja gentil.\n';
    expect(dividirEmSecoes(conteudo)).toEqual([
      { titulo: 'Horário', corpo: 'Só liga de dia.\n\n' },
      { titulo: 'Tom', corpo: 'Seja gentil.\n' },
    ]);
  });

  it('juntarSecoesDesfazDividirByteABytes', () => {
    const conteudo = '## Horário\nSó liga de dia.\n\n## Tom\nSeja gentil.\n';
    expect(juntarSecoes(dividirEmSecoes(conteudo))).toBe(conteudo);
  });

  it('juntarAplicaSoAEdicaoDaSecaoMudada', () => {
    const secoes = dividirEmSecoes('## Horário\nSó liga de dia.\n\n## Tom\nSeja gentil.\n');
    const editado = secoes.map((s) =>
      s.titulo === 'Tom' ? { ...s, corpo: 'Seja educado.\n' } : s,
    );
    expect(juntarSecoes(editado)).toBe('## Horário\nSó liga de dia.\n\n## Tom\nSeja educado.\n');
  });
});
