import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import {
  buscarEstado,
  criarAdministradora,
  criarCondominio,
  EstadoSchema,
  faltamIds,
  ferramentasDaSkill,
  ferramentasGerais,
  habilitacaoDe,
  PADRAO,
  salvarOrientacao,
} from './contratos';
import { estadoFalso, respostaMapeamento } from './estadoFalso';

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
    { condominio: 'vila-verde', caso: 'agendar-mudanca', habilitado: false, propria: false },
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

describe('criação', () => {
  const condominioNovo = {
    id: 'c-n',
    nome: 'N',
    administradora_id: '',
    account: '',
    company_id: '',
    tenant: '',
  };

  it.each([
    ['criarCondominio', () => criarCondominio(condominioNovo, null), 'POST /api/admin/condominio'],
    [
      'criarAdministradora',
      () => criarAdministradora({ id: 'adm-n', nome: 'N' }, null),
      'POST /api/admin/administradora',
    ],
  ])('%s createsWithPost', async (_nome, chamar, rota) => {
    const falso = responderCom({ [rota]: () => json(respostaMapeamento, 201) });
    await chamar();
    expect(falso).toHaveBeenCalledTimes(1);
  });
});

describe('escopo externo do conteúdo', () => {
  it('consultaOAlvoExatoComCompanyECondo', async () => {
    const urls: URL[] = [];
    responderCom({
      'GET /api/admin/estado': (req) => {
        urls.push(new URL(req.url));
        return json(estadoFalso);
      },
    });

    await buscarEstado('company-a', '101');
    await buscarEstado('company-b', '101');
    await buscarEstado('company-a', '*');
    await buscarEstado(null, '*');

    expect(
      urls.map((url) => [url.searchParams.get('company_id'), url.searchParams.get('condo_id')]),
    ).toEqual([
      ['company-a', '101'],
      ['company-b', '101'],
      ['company-a', null],
      [null, null],
    ]);
  });

  it.each([
    { company: 'company-a', condominio: '101', condoID: 101 },
    { company: 'company-a', condominio: '*', condoID: null },
    { company: null, condominio: '*', condoID: null },
  ])('gravaOrientacaoEm $company / $condominio', async ({ company, condominio, condoID }) => {
    let recebido: unknown;
    let companyNaURL: string | null = null;
    responderCom({
      'PUT /api/admin/orientacao': async (req) => {
        recebido = await req.json();
        companyNaURL = new URL(req.url).searchParams.get('company_id');
        return json(respostaMapeamento);
      },
    });

    await salvarOrientacao({ condominio, geral: 'Piscina às 22h.', por_caso: {} }, company);

    expect(companyNaURL).toBe(company);
    expect(recebido).toEqual({
      condominio,
      company_id: company ?? '',
      condo_id: condoID,
      geral: 'Piscina às 22h.',
      por_caso: {},
    });
  });
});
