import type { ReactNode } from 'react';
import { COR_DA_CHAVE_YAML } from '@/theme/tokens';

const LINHA_DE_COMENTARIO = /^(\s*)(#.*)$/;
const LINHA_DE_CHAVE = /^(\s*(?:-\s+)?)([\w.-]+)(:)(.*)$/;

/**
 * Realce raso de YAML: comentário em cinza, chave em roxo, resto sem cor.
 * Não é um parser; cobre o padrão `chave: valor` e `# comentário` do desenho aprovado.
 */
function realcarLinha(linha: string, indice: number, corDeComentario: string): ReactNode {
  const comentario = linha.match(LINHA_DE_COMENTARIO);
  if (comentario) {
    return (
      <span key={indice} style={{ color: corDeComentario }}>
        {linha}
      </span>
    );
  }
  const chave = linha.match(LINHA_DE_CHAVE);
  if (chave) {
    const [, antes, nome, doisPontos, resto] = chave;
    return (
      <span key={indice}>
        {antes}
        <span style={{ color: COR_DA_CHAVE_YAML }}>{nome}</span>
        {doisPontos}
        {resto}
      </span>
    );
  }
  return <span key={indice}>{linha}</span>;
}

/** Uma linha do texto vira um `<span>` realçado; o `\n` entre elas é literal para manter `pre`. */
export function realcarYaml(texto: string, corDeComentario: string): ReactNode[] {
  return texto.split('\n').flatMap((linha, indice, linhas) => {
    const elemento = realcarLinha(linha, indice, corDeComentario);
    return indice < linhas.length - 1 ? [elemento, '\n'] : [elemento];
  });
}
