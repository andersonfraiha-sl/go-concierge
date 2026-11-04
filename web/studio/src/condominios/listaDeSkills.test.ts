import { describe, expect, it } from 'vitest';
import { EstadoSchema, PADRAO } from './contratos';
import { estadoFalso } from './estadoFalso';
import {
  agruparPorCategoria,
  categoriaDe,
  contarLigadas,
  exemploDeOrientacao,
  filtrarLinhas,
  linhasDeSkills,
  origemDe,
  rotuloDePapeis,
  rotuloDoPapel,
} from './listaDeSkills';

const estado = EstadoSchema.parse(estadoFalso);
const titulos = new Map(estado.casos.map((caso) => [caso.nome, caso.titulo]));

const casoBase = {
  nome: 'caso-de-teste',
  titulo: 'Caso de teste',
  descricao: '',
  recursos: [],
  id: '',
  resumo: '',
  icone: '',
};

describe('categoriaDe', () => {
  it.each([
    { escrita: true, id: '', esperado: 'registra' },
    { escrita: true, id: 'tem-id', esperado: 'registra' },
    { escrita: false, id: 'tem-id', esperado: 'consulta' },
    { escrita: false, id: '', esperado: 'duvida' },
  ])(
    'categorizesByWriteAndCase escrita=$escrita id="$id" -> $esperado',
    ({ escrita, id, esperado }) => {
      expect(categoriaDe({ ...casoBase, escrita, id })).toBe(esperado);
    },
  );
});

describe('rotuloDoPapel', () => {
  it.each([
    ['morador', 'morador'],
    ['proprietario', 'proprietário'],
    ['sindico', 'síndico'],
    ['publico', 'público'],
    ['vigia', 'vigia'],
  ])('rotuloDoPapel(%s) -> %s', (papel, esperado) => {
    expect(rotuloDoPapel(papel)).toBe(esperado);
  });
});

describe('rotuloDePapeis', () => {
  it('returnsNullForEveryoneAllowed', () => {
    expect(rotuloDePapeis([])).toBeNull();
  });

  it('labelsOnlyRestrictedRoles', () => {
    expect(rotuloDePapeis(['sindico', 'proprietario'])).toBe('só síndico, proprietário');
  });
});

describe('origemDe', () => {
  it.each([
    { condominio: 'vila-verde', caso: 'agendar-mudanca', esperado: 'nenhuma' },
    { condominio: 'vila-verde', caso: 'reserva-do-salao', esperado: 'padrao' },
    { condominio: PADRAO, caso: 'reserva-do-salao', esperado: 'propria' },
  ])('tellsOriginOfDecision $condominio/$caso -> $esperado', ({ condominio, caso, esperado }) => {
    expect(origemDe(estado.mapeamento, condominio, caso)).toBe(esperado);
  });

  it('tellsOriginOfDecision própria quando o condomínio decide por si', () => {
    const mapeamento = structuredClone(estado.mapeamento);
    mapeamento.habilitacoes['vila-verde'] = {
      'agendar-mudanca': { habilitado: false, papeis: [] },
    };

    expect(origemDe(mapeamento, 'vila-verde', 'agendar-mudanca')).toBe('propria');
  });
});

describe('linhasDeSkills, filtrarLinhas, agruparPorCategoria e contarLigadas', () => {
  const linhas = linhasDeSkills(estado, 'vila-verde', titulos);

  it('filtersBySearchOnTitleAndSummary', () => {
    expect(
      filtrarLinhas(linhas, { busca: 'mudanca', soQueMuda: false }).map((l) => l.caso.nome),
    ).toEqual(['agendar-mudanca']);
    expect(
      filtrarLinhas(linhas, { busca: 'salão de festas', soQueMuda: false }).map((l) => l.caso.nome),
    ).toEqual(['reserva-do-salao']);
  });

  it('keepsOnlyOwnDecisions', () => {
    const comUmaDecisaoPropria = linhas.map((linha) =>
      linha.caso.nome === 'agendar-mudanca' ? { ...linha, propria: true } : linha,
    );

    expect(
      filtrarLinhas(comUmaDecisaoPropria, { busca: '', soQueMuda: true }).map((l) => l.caso.nome),
    ).toEqual(['agendar-mudanca']);
  });

  it('groupsInFixedOrderDroppingEmpty', () => {
    // Os dois casos da fixture escrevem: só o grupo "registra" aparece, consulta/duvida somem.
    expect(agruparPorCategoria(linhas).map((grupo) => grupo.categoria)).toEqual(['registra']);
  });

  it('countsEnabled', () => {
    expect(contarLigadas(linhas)).toEqual({ ligadas: 1, total: 2 });
  });
});

describe('exemploDeOrientacao', () => {
  it.each([
    {
      nome: 'reserva-do-salao',
      esperado: 'a churrasqueira só pode ser reservada por quem está em dia com o condomínio',
    },
    {
      nome: 'controle-de-visitante',
      esperado: 'prestadores só entram com documento; a portaria liga para a unidade antes',
    },
    {
      nome: 'agendar-mudanca',
      esperado: 'o elevador de serviço precisa de proteção; o zelador acompanha',
    },
    { nome: 'autorizar-obra', esperado: 'obras só com o formulário da administração assinado' },
    {
      nome: 'retirada-de-encomenda',
      esperado: 'a portaria guarda encomendas por até 7 dias',
    },
    { nome: 'faq', esperado: 'o que o concierge deve saber ou fazer diferente aqui' },
  ])('picksGuidanceExampleByName($nome) -> $esperado', ({ nome, esperado }) => {
    expect(exemploDeOrientacao(nome)).toBe(esperado);
  });
});
