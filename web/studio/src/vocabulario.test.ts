import { describe, expect, it } from 'vitest';

// Spec §4: fora da área Avançado, a tela só fala a língua do Studio.
const TERMOS_PROIBIDOS: readonly RegExp[] = [
  /\bBackdoor\b/,
  /\bAdmin\b/,
  /caso de uso/i,
  /subagente/i,
  /\boperador\b/i,
  /orquestrador/i,
  /roteador/i,
  /proje[çc][ãa]o/i,
  /orquestra[çc][ãa]o/i,
  /caso\.yaml/,
  /principal\.md/,
  /persona\.md/,
  /\brules\//,
  /\bknowledge\//,
  /\bNexus\b/,
  /\bMCP\b/,
  /\bYAML\b/i,
];

const telas = import.meta.glob<string>('./**/*.tsx', { query: '?raw', import: 'default', eager: true });

const telasForaDoAvancado = Object.entries(telas).filter(
  ([arquivo]) => !arquivo.startsWith('./avancado/') && !arquivo.endsWith('.test.tsx'),
);

describe('vocabulário das telas', () => {
  it('encontraAsTelas', () => {
    expect(telasForaDoAvancado.length).toBeGreaterThan(0);
  });

  it.each(telasForaDoAvancado)('%s não usa termo proibido', (arquivo, conteudo) => {
    const usados = TERMOS_PROIBIDOS.filter((termo) => termo.test(conteudo)).map((termo) => `${arquivo}: ${termo}`);
    expect(usados).toEqual([]);
  });
});
