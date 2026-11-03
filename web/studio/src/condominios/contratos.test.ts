import { describe, expect, it } from 'vitest';
import {
  EstadoSchema,
  faltamIds,
  ferramentasDaSkill,
  ferramentasGerais,
  habilitacaoDe,
  PADRAO,
} from './contratos';
import { estadoFalso } from './estadoFalso';

describe('contratos de condomínios', () => {
  it('leOEstadoDoAdmin', () => {
    const estado = EstadoSchema.parse(estadoFalso);
    expect(PADRAO).toBe('*');
    expect(estado.mapeamento.condominios[0]?.nome).toBe('Vila Verde');
    expect(estado.casos).toHaveLength(2);
    expect(estado.recursos[0]?.definicoes.map((d) => d.name)).toEqual([
      'antecedencia_dias',
      'turno',
      'exige_protocolo',
    ]);
  });

  it('entregaListasEMapasQuandoOGoMandaNulo', () => {
    const estado = EstadoSchema.parse({
      ...estadoFalso,
      mapeamento: {
        administradoras: null,
        condominios: null,
        habilitacoes: null,
        payloads: null,
        orientacoes: { '*': { geral: 'oi', por_caso: null } },
      },
      administradoras: null,
      casos: null,
      recursos: null,
    });
    expect(estado.mapeamento.condominios).toEqual([]);
    expect(estado.mapeamento.habilitacoes).toEqual({});
    expect(estado.mapeamento.orientacoes['*']?.por_caso).toEqual({});
    expect(estado.casos).toEqual([]);
    expect(estado.administradoras).toEqual([]);
  });

  it('entregaPapeisVaziosNaHabilitacaoSemPapeis', () => {
    const estado = EstadoSchema.parse(estadoFalso);
    expect(estado.mapeamento.habilitacoes['*']?.['reserva-do-salao']).toEqual({
      habilitado: false,
      papeis: [],
    });
  });
});

describe('habilitacaoDe', () => {
  const mapeamento = EstadoSchema.parse(estadoFalso).mapeamento;

  it.each([
    { condominio: 'vila-verde', caso: 'agendar-mudanca', habilitado: true, propria: false },
    { condominio: 'vila-verde', caso: 'reserva-do-salao', habilitado: false, propria: false },
    { condominio: PADRAO, caso: 'reserva-do-salao', habilitado: false, propria: true },
  ])('resolve $caso em $condominio', ({ condominio, caso, habilitado, propria }) => {
    expect(habilitacaoDe(mapeamento, condominio, caso)).toMatchObject({
      habilitacao: { habilitado },
      propria,
    });
  });
});

describe('faltamIds', () => {
  it('flagsMissingIntegrationIds', () => {
    const condominio = EstadoSchema.parse(estadoFalso).mapeamento.condominios[0];
    if (!condominio) throw new Error('fixture sem condomínio');
    expect(faltamIds(condominio)).toBe(false);
    expect(faltamIds({ ...condominio, tenant: '  ' })).toBe(true);
    expect(faltamIds({ ...condominio, account: '' })).toBe(true);
  });
});

describe('ferramentasDaSkill e ferramentasGerais', () => {
  const estado = EstadoSchema.parse(estadoFalso);

  it('splitsSkillAndGeneralTools', () => {
    const agendarMudanca = estado.casos.find((caso) => caso.nome === 'agendar-mudanca');
    if (!agendarMudanca) throw new Error('fixture sem caso agendar-mudanca');

    expect(ferramentasDaSkill(estado, agendarMudanca).map((f) => f.name)).toEqual([
      'agenda-mudanca',
    ]);
    expect(ferramentasGerais(estado).map((f) => f.name)).toEqual(['horario-portaria']);
  });
});

describe('CasoSchema', () => {
  it('defaultsSummaryAndIconWhenGoOmits', () => {
    // Simula o Go sem os campos novos: object literal fora do tipo de Caso de propósito.
    const bruto = structuredClone(estadoFalso) as unknown as { casos: Record<string, unknown>[] };
    const [primeiro, ...resto] = bruto.casos;
    if (!primeiro) throw new Error('fixture sem casos');
    const { resumo: _resumo, icone: _icone, id: _id, ...semResumoEIcone } = primeiro;
    bruto.casos = [semResumoEIcone, ...resto];

    const estado = EstadoSchema.parse(bruto);

    expect(estado.casos[0]).toMatchObject({ resumo: '', icone: '', id: '' });
  });
});
