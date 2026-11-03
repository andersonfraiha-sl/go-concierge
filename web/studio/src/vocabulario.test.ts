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

// Task 3: em toda tela, mesmo no Avançado, o robô chama-se Concierge.
const NOME_ANTIGO: readonly RegExp[] = [/\bAssistentes?\b/, /\b(a|da|na|pela|à) assistente\b/i];

const telas = import.meta.glob<string>('./**/*.tsx', {
  query: '?raw',
  import: 'default',
  eager: true,
});

const telasForaDoAvancado = Object.entries(telas).filter(
  ([arquivo]) => !arquivo.startsWith('./avancado/') && !arquivo.endsWith('.test.tsx'),
);

const todasAsTelas = Object.entries(telas).filter(([arquivo]) => !arquivo.endsWith('.test.tsx'));

describe('vocabulário das telas', () => {
  it('encontraAsTelas', () => {
    expect(telasForaDoAvancado.length).toBeGreaterThan(0);
  });

  it.each(telasForaDoAvancado)('%s não usa termo proibido', (arquivo, conteudo) => {
    const usados = TERMOS_PROIBIDOS.filter((termo) => termo.test(conteudo)).map(
      (termo) => `${arquivo}: ${termo}`,
    );
    expect(usados).toEqual([]);
  });

  it.each(todasAsTelas)('%s chama o robô de Concierge', (arquivo, conteudo) => {
    const usados = NOME_ANTIGO.filter((termo) => termo.test(conteudo)).map(
      (termo) => `${arquivo}: ${termo}`,
    );
    expect(usados).toEqual([]);
  });
});
