import { MarkdownManager } from '@tiptap/markdown';
import { getSchema, type JSONContent, resolveExtensions } from '@tiptap/react';
import { EXTENSOES } from './extensoes';

/** Um bloco do arquivo como foi lido: o texto exato e o que o separa do próximo. */
type Bloco = { texto: string; depois: string; chave: string };

/** Markdown aberto no editor: o documento e os blocos de origem, para regravar sem mudar nada. */
export type Aberto = { doc: JSONContent; blocos: readonly Bloco[]; antes: string };

// Frontmatter, tabela, HTML (além do <u> do sublinhado) e lista que começa em 0 o editor não
// representa: fica o texto puro. Código entre crases não conta, `<id>` ali é texto.
const SINTAXE_SEM_LUGAR = [
  /^---\n/,
  /^ {0,3}\|.*\|[ \t]*$/m,
  /<(?!\/?u>)[A-Za-z!/?]/,
  /^ *0+[.)] /m,
];
const semCodigoEmLinha = (markdown: string): string => markdown.replace(/`[^`\n]*`/g, '');
const BLOCOS_DO_EDITOR = new Set(['heading', 'paragraph', 'list', 'code', 'blockquote', 'hr']);

// Só o que viraria formatação ao reabrir; `&`, `>`, `[` e o `_` dentro de palavra ficam como estão.
const escaparTexto = (texto: string): string =>
  texto
    .replace(/[\\`*]|~~/g, (achado) => achado.replace(/./g, '\\$&'))
    .replace(/(?<![\p{L}\p{N}])_|_(?![\p{L}\p{N}])/gu, '\\_')
    .replace(/<(?=[A-Za-z/!?])/g, '\\<');

const estaEmCodigo = (no: JSONContent, pai: JSONContent | undefined): boolean =>
  pai?.type === 'codeBlock' || (no.marks ?? []).some((marca) => marca.type === 'code');

const conversor = new MarkdownManager({ extensions: resolveExtensions(EXTENSOES) });
// ponytail: troca o escape interno do conversor (que vira `&` em `&amp;` e `_` em `\_`); se a
// versão do @tiptap/markdown mudar, os testes de ida e volta denunciam.
Object.assign(conversor, {
  encodeTextForMarkdown: (texto: string, no: JSONContent, pai?: JSONContent) =>
    estaEmCodigo(no, pai) ? texto : escaparTexto(texto),
});
const esquema = getSchema(EXTENSOES);

/** Linha de continuação dentro de um item fica alinhada ao texto do item ("- " ou "10. "). */
const larguraDoMarcador = (lista: JSONContent, indice: number): number =>
  lista.type === 'orderedList' ? String(Number(lista.attrs?.start ?? 1) + indice).length + 2 : 2;

const eLista = (no: JSONContent): boolean => no.type === 'bulletList' || no.type === 'orderedList';

/** Aplica `trocar` às quebras de linha do texto corrido, sabendo o recuo do item onde ele está. */
const trocarQuebras = (
  no: JSONContent,
  trocar: (texto: string, recuo: number) => string,
  recuo = 0,
): JSONContent => {
  if (no.type === 'codeBlock') return no;
  if (no.type === 'text') {
    if (no.text === undefined || estaEmCodigo(no, undefined)) return no;
    return { ...no, text: trocar(no.text, recuo) };
  }
  const content = no.content?.map((filho, indice) =>
    trocarQuebras(filho, trocar, eLista(no) ? larguraDoMarcador(no, indice) : recuo),
  );
  return content === undefined ? no : { ...no, content };
};

// O recuo no começo da linha de continuação não muda o parágrafo: some na leitura, volta na escrita.
const semRecuo = (texto: string): string => texto.replace(/\n[ \t]+/g, '\n');
const comRecuo = (texto: string, recuo: number): string =>
  texto.replaceAll('\n', `\n${' '.repeat(recuo)}`);

const chaveDe = (no: JSONContent): string => JSON.stringify(esquema.nodeFromJSON(no).toJSON());
const blocosDe = (markdown: string): JSONContent[] =>
  (conversor.parse(markdown).content ?? []).map((no) => trocarQuebras(no, semRecuo));
const naoVazio = (no: JSONContent): boolean =>
  !(no.type === 'paragraph' && (no.content ?? []).length === 0);
const serializarBloco = (no: JSONContent): string =>
  conversor.serialize({ type: 'doc', content: [trocarQuebras(no, comRecuo)] }).replace(/\n+$/, '');

/** O bloco volta igual depois de passar pelo editor: editar não muda o que ele significa. */
const voltaIgual = (no: JSONContent): boolean => {
  const [relido] = blocosDe(serializarBloco(no));
  return relido !== undefined && chaveDe(relido) === chaveDe(no);
};

type Token = { type: string; raw: string };

type BlocoLido = { bloco: Bloco; no: JSONContent };

/** Um bloco do arquivo, ou `null` se o editor não o representa sem perda. */
function lerBloco(token: Token): BlocoLido | null {
  if (!BLOCOS_DO_EDITOR.has(token.type)) return null;
  const texto = token.raw.replace(/\n+$/, '');
  const [no, ...sobra] = blocosDe(texto);
  if (no === undefined || sobra.length > 0 || !voltaIgual(no)) return null;
  return { bloco: { texto, depois: token.raw.slice(texto.length), chave: chaveDe(no) }, no };
}

/** Separa o arquivo em blocos que o editor representa sem perda; `null` se algum não couber. */
function lerBlocos(
  tokens: readonly Token[],
): (Omit<Aberto, 'doc'> & { nos: JSONContent[] }) | null {
  const blocos: Bloco[] = [];
  const nos: JSONContent[] = [];
  let antes = '';
  for (const token of tokens) {
    const anterior = blocos.at(-1);
    if (token.type === 'space') {
      if (anterior === undefined) antes += token.raw;
      else anterior.depois += token.raw;
      continue;
    }
    const lido = lerBloco(token);
    if (lido === null) return null;
    blocos.push(lido.bloco);
    nos.push(lido.no);
  }
  return { blocos, nos, antes };
}

/**
 * Abre o Markdown no editor, ou `null` quando o editor não o representa sem perda
 * (frontmatter, tabela, HTML, bloco que não volta igual): aí o campo fica em texto puro.
 */
export function abrir(markdown: string): Aberto | null {
  if (SINTAXE_SEM_LUGAR.some((sintaxe) => sintaxe.test(semCodigoEmLinha(markdown)))) return null;
  const tokens = conversor.instance.lexer(markdown);
  if (tokens.map((token) => token.raw).join('') !== markdown) return null;
  const lidos = lerBlocos(tokens);
  if (lidos === null) return null;
  // Lido bloco a bloco tem de dar o mesmo que o arquivo inteiro (o que o concierge entende).
  const noContexto = blocosDe(markdown).filter(naoVazio).map(chaveDe);
  if (noContexto.join() !== lidos.blocos.map((bloco) => bloco.chave).join()) return null;
  return { doc: { type: 'doc', content: lidos.nos }, blocos: lidos.blocos, antes: lidos.antes };
}

type Parte = { texto: string; indice: number | null };

/** Bloco que não mudou volta com o texto exato de origem; só o que foi editado é regravado. */
export function salvar(doc: JSONContent, aberto: Aberto): string {
  const usados = new Set<number>();
  const partes = (doc.content ?? []).filter(naoVazio).map((no): Parte => {
    const chave = chaveDe(no);
    const indice = aberto.blocos.findIndex((bloco, i) => !usados.has(i) && bloco.chave === chave);
    const origem = aberto.blocos[indice];
    if (origem === undefined) return { texto: serializarBloco(no), indice: null };
    usados.add(indice);
    return { texto: origem.texto, indice };
  });
  if (partes.length === 0) return '';
  const final = aberto.blocos.at(-1)?.depois ?? '';
  const separador = (parte: Parte, proxima: Parte | undefined): string => {
    if (proxima === undefined) return final;
    if (parte.indice === null || proxima.indice !== parte.indice + 1) return '\n\n';
    return aberto.blocos[parte.indice]?.depois ?? '\n\n';
  };
  return (
    aberto.antes + partes.map((parte, i) => parte.texto + separador(parte, partes[i + 1])).join('')
  );
}
