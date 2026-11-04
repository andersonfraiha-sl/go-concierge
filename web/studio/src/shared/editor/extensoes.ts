import { type JSONContent, Mark } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';

const SUBLINHADO = /^<u>([\s\S]+?)<\/u>/;

/**
 * O sublinhado do StarterKit grava `++texto++`; aqui ele é `<u>texto</u>`, HTML que o Markdown
 * já aceita, para o arquivo e o prompt do concierge não mudarem de formato.
 */
const Sublinhado = Mark.create({
  name: 'underline',
  parseHTML: () => [{ tag: 'u' }, { style: 'text-decoration=underline' }],
  renderHTML: () => ['u', 0],
  addKeyboardShortcuts() {
    return { 'Mod-u': () => this.editor.commands.toggleMark(this.name) };
  },
  markdownTokenName: 'underline',
  parseMarkdown: (token, helpers) =>
    helpers.applyMark('underline', helpers.parseInline(token.tokens ?? [])),
  renderMarkdown: (node: JSONContent, helpers) => `<u>${helpers.renderChildren(node)}</u>`,
  markdownTokenizer: {
    name: 'underline',
    level: 'inline',
    start: (src: string) => src.indexOf('<u>'),
    tokenize(src, _tokens, lexer) {
      const achado = SUBLINHADO.exec(src);
      if (achado === null) return undefined;
      const interno = achado[1] ?? '';
      return {
        type: 'underline',
        raw: achado[0],
        text: interno,
        tokens: lexer.inlineTokens(interno),
      };
    },
  },
});

/** Link fica de fora: URL solta continua texto, em vez de virar `[url](url)` ao salvar. */
export const EXTENSOES = [StarterKit.configure({ underline: false, link: false }), Sublinhado];
