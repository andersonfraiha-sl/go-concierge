import { describe, expect, it } from 'vitest';
import { EstadoSchema, habilitacaoDe, PADRAO } from './contratos';
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
