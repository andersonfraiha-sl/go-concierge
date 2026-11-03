import { contemBusca } from '@/shared/busca';
import {
  type Caso,
  type Estado,
  type Habilitacao,
  habilitacaoDe,
  type Mapeamento,
  PADRAO,
} from './contratos';

export type Categoria = 'registra' | 'consulta' | 'duvida';

export const ROTULO_DA_CATEGORIA: Record<Categoria, string> = {
  registra: 'Registra pedidos',
  consulta: 'Consultas',
  duvida: 'Dúvidas',
};

const ORDEM_DAS_CATEGORIAS: readonly Categoria[] = ['registra', 'consulta', 'duvida'];

/** Escreve algo → registra pedido; só lê mas tem id de integração → consulta; senão, é dúvida de conhecimento. */
export function categoriaDe(caso: Caso): Categoria {
  if (caso.escrita) return 'registra';
  if (caso.id !== '') return 'consulta';
  return 'duvida';
}

const ROTULO_DO_PAPEL: Record<string, string> = {
  morador: 'morador',
  proprietario: 'proprietário',
  sindico: 'síndico',
  publico: 'público',
};

/** Papel sem rótulo conhecido mostra o próprio valor. */
export function rotuloDoPapel(papel: string): string {
  return ROTULO_DO_PAPEL[papel] ?? papel;
}

export function rotuloDePapeis(papeis: readonly string[]): string | null {
  if (papeis.length === 0) return null;
  return `só ${papeis.map(rotuloDoPapel).join(', ')}`;
}

export type Origem = 'propria' | 'padrao' | 'nenhuma';

/** Decisão do condomínio → propria; senão, fora do Padrão com decisão no Padrão → padrao; senão nenhuma. */
export function origemDe(mapeamento: Mapeamento, condominio: string, caso: string): Origem {
  if (mapeamento.habilitacoes[condominio]?.[caso]) return 'propria';
  if (condominio !== PADRAO && mapeamento.habilitacoes[PADRAO]?.[caso]) return 'padrao';
  return 'nenhuma';
}

export type LinhaDeSkill = {
  caso: Caso;
  titulo: string;
  habilitacao: Habilitacao;
  propria: boolean;
  categoria: Categoria;
};

export function linhasDeSkills(
  estado: Estado,
  condominio: string,
  titulos: ReadonlyMap<string, string>,
): LinhaDeSkill[] {
  return estado.casos.map((caso) => {
    const { habilitacao, propria } = habilitacaoDe(estado.mapeamento, condominio, caso.nome);
    return {
      caso,
      titulo: titulos.get(caso.nome) ?? caso.titulo,
      habilitacao,
      propria,
      categoria: categoriaDe(caso),
    };
  });
}

export function filtrarLinhas(
  linhas: readonly LinhaDeSkill[],
  filtro: { busca: string; soQueMuda: boolean },
): LinhaDeSkill[] {
  return linhas.filter(
    (linha) =>
      contemBusca([linha.titulo, linha.caso.resumo], filtro.busca) &&
      (!filtro.soQueMuda || linha.propria),
  );
}

export function agruparPorCategoria(
  linhas: readonly LinhaDeSkill[],
): { categoria: Categoria; linhas: LinhaDeSkill[] }[] {
  return ORDEM_DAS_CATEGORIAS.map((categoria) => ({
    categoria,
    linhas: linhas.filter((linha) => linha.categoria === categoria),
  })).filter((grupo) => grupo.linhas.length > 0);
}

export function contarLigadas(linhas: readonly LinhaDeSkill[]): { ligadas: number; total: number } {
  return {
    ligadas: linhas.filter((linha) => linha.habilitacao.habilitado).length,
    total: linhas.length,
  };
}

const EXEMPLOS_DE_ORIENTACAO: readonly [RegExp, string][] = [
  [/reserva/, 'a churrasqueira só pode ser reservada por quem está em dia com o condomínio'],
  [/visitante/, 'prestadores só entram com documento; a portaria liga para a unidade antes'],
  [/mudanca/, 'o elevador de serviço precisa de proteção; o zelador acompanha'],
  [/obra/, 'obras só com o formulário da administração assinado'],
  [/encomenda|retirada/, 'a portaria guarda encomendas por até 7 dias'],
];

const EXEMPLO_GENERICO = 'o que o concierge deve saber ou fazer diferente aqui';

export function exemploDeOrientacao(nomeDaSkill: string): string {
  const casado = EXEMPLOS_DE_ORIENTACAO.find(([regex]) => regex.test(nomeDaSkill));
  return casado?.[1] ?? EXEMPLO_GENERICO;
}
