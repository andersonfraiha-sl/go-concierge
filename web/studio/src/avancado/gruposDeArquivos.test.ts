import { describe, expect, it } from 'vitest';
import type { Arquivo } from '@/catalogo/contratos';
import { agruparArquivos } from './gruposDeArquivos';

const arquivo = (caminho: string): Arquivo => ({ caminho, bytes: 1 });

describe('agruparArquivos', () => {
  it.each([
    { pasta: 'agent.yaml', grupo: 'Raiz' },
    { pasta: 'persona.md', grupo: 'Raiz' },
    { pasta: 'caso.yaml', grupo: 'Raiz' },
    { pasta: 'rules', grupo: 'Raiz' },
    { pasta: 'workflows', grupo: 'Raiz' },
    { pasta: 'knowledge', grupo: 'Conhecimento' },
    { pasta: 'routers', grupo: 'Rotas' },
    { pasta: 'assets', grupo: 'Modelos e testes' },
    { pasta: 'evals', grupo: 'Modelos e testes' },
  ])('groupsFilesByFolder: $pasta vai para $grupo', ({ pasta, grupo }) => {
    const resultado = agruparArquivos({ [pasta]: [arquivo(`${pasta}/x`)] });
    expect(resultado).toEqual([{ grupo, arquivos: [arquivo(`${pasta}/x`)] }]);
  });

  it('groupsFilesByFolder combines multiple folders in the same group in a fixed order', () => {
    const resultado = agruparArquivos({
      evals: [arquivo('evals/casos.yaml')],
      rules: [arquivo('rules/politicas.md')],
      assets: [arquivo('assets/rodape.md')],
    });
    expect(resultado).toEqual([
      { grupo: 'Raiz', arquivos: [arquivo('rules/politicas.md')] },
      {
        grupo: 'Modelos e testes',
        arquivos: [arquivo('evals/casos.yaml'), arquivo('assets/rodape.md')],
      },
    ]);
  });

  it('groupsFilesByFolder omits groups without files', () => {
    const resultado = agruparArquivos({ knowledge: [] });
    expect(resultado).toEqual([]);
  });

  it('groupsFilesByFolder ignores unknown folders', () => {
    const resultado = agruparArquivos({ desconhecida: [arquivo('desconhecida/x')] });
    expect(resultado).toEqual([]);
  });
});
