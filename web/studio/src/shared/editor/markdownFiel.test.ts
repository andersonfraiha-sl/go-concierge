import { readdirSync, readFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { Editor } from '@tiptap/react';
import { describe, expect, it } from 'vitest';
import { EXTENSOES } from './extensoes';
import { abrir, salvar } from './markdownFiel';

const CATALOGO = resolve(import.meta.dirname, '../../../../../okf/agentes');

const arquivosMarkdown = (pasta: string): string[] =>
  readdirSync(pasta, { withFileTypes: true }).flatMap((item) => {
    const caminho = join(pasta, item.name);
    if (item.isDirectory()) return arquivosMarkdown(caminho);
    return item.name.endsWith('.md') ? [caminho] : [];
  });

/** O que o editor faz com o documento ao montar (atributos padrão, normalização do esquema). */
const passarPeloEditor = (markdown: string): string | null => {
  const aberto = abrir(markdown);
  if (aberto === null) return null;
  const editor = new Editor({ extensions: EXTENSOES, content: aberto.doc });
  const salvo = salvar(editor.getJSON(), aberto);
  editor.destroy();
  return salvo;
};

// Só o que o editor não representa pode ficar de fora: frontmatter e lista que começa em 0.
const temSintaxeSemLugar = (markdown: string): boolean =>
  markdown.startsWith('---\n') || /^ *0\. /m.test(markdown);

describe('markdownFiel', () => {
  const arquivos = arquivosMarkdown(CATALOGO).map((caminho) => ({
    nome: caminho.slice(CATALOGO.length + 1).replaceAll('\\', '/'),
    conteudo: readFileSync(caminho, 'utf8'),
  }));

  it('findsTheRealSkillFiles', () => {
    expect(arquivos.length).toBeGreaterThan(30);
  });

  it.each(arquivos)('returnsRealFileByteForByte: $nome', ({ conteudo }) => {
    const salvo = passarPeloEditor(conteudo);
    if (salvo === null) expect(temSintaxeSemLugar(conteudo)).toBe(true);
    else expect(salvo).toBe(conteudo);
  });

  it.each([
    { caso: 'sublinhado', markdown: 'Leia <u>com atenção</u>.\n' },
    { caso: 'sublinhado em negrito', markdown: '**<u>Nunca</u>** invente.\n' },
    { caso: 'sublinhado de dentro do código', markdown: 'Use `chamar_recurso` e `<id>`.\n' },
    { caso: 'caracteres sem escape', markdown: 'A & B > C, [código], 50% e snake_case.\n' },
    { caso: 'sem quebra no fim', markdown: '## Título\nTexto colado.' },
    { caso: 'linhas em branco extras', markdown: '\n# A\n\n\n\nB\n\n' },
    { caso: 'lista com continuação', markdown: '1. Um\n   dois\n2. Três\n\n- a\n  b\n' },
  ])('returnsByteForByte: $caso', ({ markdown }) => {
    expect(passarPeloEditor(markdown)).toBe(markdown);
  });

  it.each([
    { caso: 'tabela', markdown: '| a | b |\n|---|---|\n| 1 | 2 |\n' },
    { caso: 'HTML', markdown: 'Texto <div>solto</div>.\n' },
    { caso: 'frontmatter', markdown: '---\nid: x\n---\n# Título\n' },
    { caso: 'fim de linha do Windows', markdown: '# A\r\n\r\nB\r\n' },
    { caso: 'lista que começa em 0', markdown: '0. Zero\n1. Um\n' },
  ])('refusesWhatItCannotRepresent: $caso', ({ markdown }) => {
    expect(abrir(markdown)).toBeNull();
  });

  it('rewritesOnlyTheEditedBlock', () => {
    const original = '# Políticas\n\n- Use `chamar_recurso`; A & B.\n  continua aqui.\n\nFim.\n';
    const aberto = abrir(original);
    if (aberto === null) throw new Error('deveria abrir');
    const editor = new Editor({ extensions: EXTENSOES, content: aberto.doc });
    editor.commands.setTextSelection({ from: 1, to: 3 });
    editor.commands.toggleBold();
    const salvo = salvar(editor.getJSON(), aberto);
    editor.destroy();
    expect(salvo).toBe(original.replace('# Políticas', '# **Po**líticas'));
  });

  it('keepsMeaningOfEditedListWithContinuation', () => {
    const aberto = abrir('1. Leia `x_y` (ler_dado) e procure\n   de novo.\n');
    if (aberto === null) throw new Error('deveria abrir');
    const editor = new Editor({ extensions: EXTENSOES, content: aberto.doc });
    editor.commands.insertContentAt(3, 'Sempre ');
    const salvo = salvar(editor.getJSON(), aberto);
    editor.destroy();
    expect(salvo).toBe('1. Sempre Leia `x_y` (ler_dado) e procure\n   de novo.\n');
  });
});
