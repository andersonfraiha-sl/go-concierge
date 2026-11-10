import { z } from 'zod';
import { BLOCOS } from '@/fluxo/modelo/catalogo';
import { type Bloco, BlocoSchema, InformacaoSchema } from '@/fluxo/modelo/tipos';
import type { TIPOS_DE_TRECHO } from './contratos';

type TipoDeTrecho = (typeof TIPOS_DE_TRECHO)[number];

/** Como a tela desenha o conteúdo canônico de um trecho. */
export type ConteudoLegivel =
  | { forma: 'texto'; texto: string }
  | { forma: 'cartao'; titulo: string; texto: string }
  | { forma: 'lista'; itens: { id: string; rotulo: string }[] };

/** Rótulo de cada etapa pelo id do trecho (`etapa:<id>`), para a ordem mostrar nomes e não ids. */
export type RotulosDasEtapas = ReadonlyMap<string, string>;

const PREFIXO_DE_ETAPA = 'etapa:';

function assertNever(valor: never): never {
  throw new Error(`bloco não tratado: ${JSON.stringify(valor)}`);
}

function textoPrincipalDoBloco(bloco: Bloco): string {
  switch (bloco.tipo) {
    case 'buscar_dados':
    case 'gravar_no_sistema':
      return bloco.recurso;
    case 'pedir_escolha':
      return bloco.pergunta || bloco.titulo;
    case 'perguntar':
      return bloco.campos.join(', ');
    case 'formulario':
      return bloco.de;
    case 'reaproveitar':
      return bloco.pergunta || bloco.campos.join(', ');
    case 'confirmar':
    case 'montar_texto':
    case 'responder':
      return bloco.texto;
    default:
      return assertNever(bloco);
  }
}

// O conteúdo vem do servidor como texto JSON. Se não for JSON válido, a pessoa ainda lê o texto cru.
function lerJson(bruto: string): unknown {
  try {
    return JSON.parse(bruto);
  } catch {
    return undefined;
  }
}

function legivelDaEtapa(bruto: string): ConteudoLegivel | null {
  const bloco = BlocoSchema.safeParse(lerJson(bruto));
  if (!bloco.success) return null;
  return {
    forma: 'cartao',
    titulo: BLOCOS[bloco.data.tipo].rotulo,
    texto: textoPrincipalDoBloco(bloco.data),
  };
}

function legivelDaInformacao(bruto: string): ConteudoLegivel | null {
  const informacao = InformacaoSchema.safeParse(lerJson(bruto));
  if (!informacao.success) return null;
  const { titulo, pergunta, nome } = informacao.data;
  return { forma: 'cartao', titulo: titulo || nome, texto: pergunta };
}

function legivelDaOrdem(bruto: string, rotulos: RotulosDasEtapas): ConteudoLegivel | null {
  const ids = z.array(z.string()).safeParse(lerJson(bruto));
  if (!ids.success) return null;
  return {
    forma: 'lista',
    itens: ids.data.map((id) => ({ id, rotulo: rotulos.get(`${PREFIXO_DE_ETAPA}${id}`) ?? id })),
  };
}

export function lerConteudo(
  tipo: TipoDeTrecho,
  bruto: string,
  rotulos: RotulosDasEtapas,
): ConteudoLegivel {
  const legivel = {
    texto: () => null,
    orientacao: () => null,
    etapa: () => legivelDaEtapa(bruto),
    informacao: () => legivelDaInformacao(bruto),
    ordem: () => legivelDaOrdem(bruto, rotulos),
  }[tipo]();
  return legivel ?? { forma: 'texto', texto: bruto };
}

/** Bloco guardado no conteúdo de uma etapa; `null` se o conteúdo não for uma etapa válida. */
export function blocoDoConteudo(bruto: string | null): Bloco | null {
  if (bruto === null) return null;
  const bloco = BlocoSchema.safeParse(lerJson(bruto));
  return bloco.success ? bloco.data : null;
}
