import { describe, expect, it, vi } from 'vitest';
import { EuSchema } from '@/acesso/eu';
import { EstadoSchema } from '@/condominios/contratos';
import { json, responderCom } from '@/test/fetchFalso';
import {
  type Ambiente,
  aplicarOperacao,
  operacaoDaVez,
  podeAplicar,
  TextosNaoSalvaram,
} from './aplicar';
import { OperacaoSchema } from './contratos';

const estado = EstadoSchema.parse({
  mapeamento: {
    administradoras: [],
    condominios: [],
    habilitacoes: { 'vila-verde': { encomendas: { habilitado: false, papeis: ['sindico'] } } },
    payloads: { 'vila-verde': { 'reserva.criar': { antecedencia: 2 }, 'outro.recurso': { x: 1 } } },
    orientacoes: {
      'vila-verde': { geral: 'Piscina até 22h.', por_caso: { mudanca: 'Sem domingo.' } },
    },
  },
  administradoras: [],
  casos: [],
  recursos: [],
  papeis: [],
});

const textosLidos = {
  caminho: 'agentes/concierge/agentes/agendar-mudanca',
  versao: 'v7',
  textos: {
    titulo: 'Agendar mudança',
    quandoUsar: 'Quando o morador quer mudar.',
    frasesDeExemplo: ['Quero mudar'],
    palavrasChave: ['mudança'],
    temFluxo: true,
    resumo: 'Agenda a mudança.',
    visibilidade: '',
  },
};

const fluxo = { blocos: [{ id: 'b1', tipo: 'responder', texto: 'Oi' }], informacoes: [] };

function ambiente(sobre: Partial<Ambiente> = {}): Ambiente {
  return { escopo: null, lerEstado: async () => estado, confirmar: async () => true, ...sobre };
}

/** Rota que guarda o corpo recebido e responde `resposta`. */
function gravando(resposta: unknown, status = 200) {
  const corpos: unknown[] = [];
  const rota = async (req: Request) => {
    corpos.push(await req.json());
    return json(resposta, status);
  };
  return { corpos, rota };
}

const mapeamento = { mapeamento: estado.mapeamento, administradoras: [] };
const op = (bruta: unknown) => OperacaoSchema.parse(bruta);

describe('aplicarOperacao', () => {
  it('appliesTextsWithCurrentVersion', async () => {
    const put = gravando({ versao: 'v8' });
    responderCom({
      'GET /api/skills/agendar-mudanca': () => json(textosLidos),
      'PUT /api/skills/agendar-mudanca/textos': put.rota,
    });

    await aplicarOperacao(
      op({ tipo: 'preencher_textos', skill: 'agendar-mudanca', textos: { resumo: 'Novo.' } }),
      ambiente(),
    );

    expect(put.corpos).toEqual([
      { versao: 'v7', textos: { ...textosLidos.textos, resumo: 'Novo.' } },
    ]);
  });

  it('explainsVersionConflictOnTexts', async () => {
    responderCom({
      'GET /api/skills/agendar-mudanca': () => json(textosLidos),
      'PUT /api/skills/agendar-mudanca/textos': () => json({ codigo: 'VERSAO' }, 409),
    });

    await expect(
      aplicarOperacao(
        op({ tipo: 'preencher_textos', skill: 'agendar-mudanca', textos: { resumo: 'X' } }),
        ambiente(),
      ),
    ).rejects.toThrow('Esta skill mudou desde a proposta. Peça de novo ao Copiloto.');
  });

  it('refusesFlowRejectedByServer', async () => {
    const put = gravando(
      {
        codigo: 'FLUXO_INVALIDO',
        mensagem: 'O fluxo tem problemas; nada foi salvo.',
        problemas: [
          {
            codigo: 'SEM_CONFIRMAR',
            nivel: 'erro',
            mensagem: 'Gravar precisa de Confirmar antes.',
          },
        ],
      },
      422,
    );
    responderCom({
      'GET /api/skills/agendar-mudanca/fluxo': () => json({ fluxo, versao: 'f3', problemas: [] }),
      'PUT /api/skills/agendar-mudanca/fluxo': put.rota,
    });

    await expect(
      aplicarOperacao(op({ tipo: 'trocar_fluxo', skill: 'agendar-mudanca', fluxo }), ambiente()),
    ).rejects.toThrow(
      'O fluxo tem problemas; nada foi salvo.\n• Gravar precisa de Confirmar antes.',
    );
    expect(put.corpos).toEqual([{ versao: 'f3', fluxo }]);
  });

  it('asksBeforeDeletingSkill', async () => {
    const confirmar = vi.fn(async () => false);
    const fetchFalso = responderCom({
      'GET /api/skills/agendar-mudanca': () => json(textosLidos),
    });

    const resultado = await aplicarOperacao(
      op({ tipo: 'excluir_skill', skill: 'agendar-mudanca' }),
      ambiente({ confirmar }),
    );

    expect(resultado).toBe('desistiu');
    expect(confirmar).toHaveBeenCalledWith(
      'Excluir "Agendar mudança"? O fluxo e as regras somem junto.',
    );
    expect(fetchFalso.mock.calls.some(([, init]) => init?.method === 'DELETE')).toBe(false);
  });

  it('deletesSkillAfterConfirmation', async () => {
    const fetchFalso = responderCom({
      'GET /api/skills/agendar-mudanca': () => json(textosLidos),
      'DELETE /api/skills/agendar-mudanca': () => new Response(null, { status: 204 }),
    });

    await aplicarOperacao(op({ tipo: 'excluir_skill', skill: 'agendar-mudanca' }), ambiente());

    expect(fetchFalso.mock.calls.some(([, init]) => init?.method === 'DELETE')).toBe(true);
  });

  it('createsSkillAndFillsItsTexts', async () => {
    const post = gravando({ nome: 'achados-e-perdidos' });
    const put = gravando({ versao: 'v2' });
    responderCom({
      'POST /api/skills': post.rota,
      'GET /api/skills/achados-e-perdidos': () => json({ ...textosLidos, versao: 'v1' }),
      'PUT /api/skills/achados-e-perdidos/textos': put.rota,
    });

    await aplicarOperacao(
      op({ tipo: 'criar_skill', titulo: 'Achados e perdidos', textos: { resumo: 'Acha.' } }),
      ambiente(),
    );

    expect(post.corpos).toEqual([{ nome: 'achados-e-perdidos', titulo: 'Achados e perdidos' }]);
    expect(put.corpos).toEqual([
      { versao: 'v1', textos: { ...textosLidos.textos, resumo: 'Acha.' } },
    ]);
  });

  it.each([
    { caso: 'longo', resumo: 'a'.repeat(91) },
    { caso: 'em duas linhas', resumo: 'Uma linha.\nOutra.' },
  ])('refusesSummaryOutsideTheRule ($caso)', async ({ resumo }) => {
    const fetchFalso = responderCom({});

    await expect(
      aplicarOperacao(op({ tipo: 'preencher_textos', skill: 's', textos: { resumo } }), ambiente()),
    ).rejects.toThrow('O resumo tem uma linha só, até 90 caracteres.');
    expect(fetchFalso).not.toHaveBeenCalled();
  });

  it('refusesCreatingSkillWithoutUsableTitle', async () => {
    const fetchFalso = responderCom({});

    await expect(
      aplicarOperacao(op({ tipo: 'criar_skill', titulo: '  !!  ' }), ambiente()),
    ).rejects.toThrow('Use letras ou números no nome.');
    expect(fetchFalso).not.toHaveBeenCalled();
  });

  it('trimsTitleWhenCreatingSkill', async () => {
    const post = gravando({ nome: 'portaria' });
    responderCom({ 'POST /api/skills': post.rota });

    await aplicarOperacao(op({ tipo: 'criar_skill', titulo: '  Portaria ' }), ambiente());

    expect(post.corpos).toEqual([{ nome: 'portaria', titulo: 'Portaria' }]);
  });

  it('reportsCreatedSkillWhenTextsFail', async () => {
    responderCom({
      'POST /api/skills': () => json({ nome: 'portaria' }),
      'GET /api/skills/portaria': () => json(textosLidos),
      'PUT /api/skills/portaria/textos': () => json({ mensagem: 'Disco cheio.' }, 500),
    });

    const falha = aplicarOperacao(
      op({ tipo: 'criar_skill', titulo: 'Portaria', textos: { resumo: 'Abre o portão.' } }),
      ambiente(),
    );

    await expect(falha).rejects.toThrow(TextosNaoSalvaram);
    await expect(falha).rejects.toMatchObject({
      nome: 'portaria',
      message: 'Skill criada, mas os textos não salvaram: Disco cheio.',
    });
  });

  it('retriesOnlyTheTextsOfCreatedSkill', () => {
    const criar = op({ tipo: 'criar_skill', titulo: 'Portaria', textos: { resumo: 'R' } });

    expect(operacaoDaVez(criar, undefined)).toBe(criar);
    expect(operacaoDaVez(criar, 'portaria')).toEqual({
      tipo: 'preencher_textos',
      skill: 'portaria',
      textos: { resumo: 'R' },
    });
  });

  it('mergesGuidanceWithCurrentState', async () => {
    const put = gravando(mapeamento);
    responderCom({ 'PUT /api/admin/orientacao': put.rota });

    await aplicarOperacao(
      op({
        tipo: 'orientar',
        condominio: 'vila-verde',
        skill: 'encomendas',
        texto: 'Na portaria.',
      }),
      ambiente(),
    );
    await aplicarOperacao(
      op({ tipo: 'orientar', condominio: 'vila-verde', texto: 'Piscina até 23h.' }),
      ambiente(),
    );

    expect(put.corpos).toEqual([
      {
        condominio: 'vila-verde',
        geral: 'Piscina até 22h.',
        por_caso: { mudanca: 'Sem domingo.', encomendas: 'Na portaria.' },
      },
      {
        condominio: 'vila-verde',
        geral: 'Piscina até 23h.',
        por_caso: { mudanca: 'Sem domingo.' },
      },
    ]);
  });

  it('mergesAdjustmentsWithCurrentState', async () => {
    const put = gravando(mapeamento);
    responderCom({ 'PUT /api/admin/ajustes': put.rota });

    await aplicarOperacao(
      op({
        tipo: 'ajustar',
        condominio: 'vila-verde',
        recurso: 'reserva.criar',
        campos: { limite: 3 },
      }),
      ambiente(),
    );

    expect(put.corpos).toEqual([
      {
        condominio: 'vila-verde',
        ajustes: {
          'reserva.criar': { antecedencia: 2, limite: 3 },
          'outro.recurso': { x: 1 },
        },
      },
    ]);
  });

  it('togglesSkillKeepingRoles', async () => {
    const put = gravando(mapeamento);
    responderCom({ 'PUT /api/admin/habilitacao': put.rota });

    await aplicarOperacao(
      op({
        tipo: 'habilitar_skill',
        condominio: 'vila-verde',
        skill: 'encomendas',
        habilitado: true,
      }),
      ambiente(),
    );

    expect(put.corpos).toEqual([
      {
        condominio: 'vila-verde',
        caso: 'encomendas',
        habilitado: true,
        papeis: ['sindico'],
        limpar: false,
      },
    ]);
  });

  it('writesPersonaFile', async () => {
    const put = gravando({ salvo: 'persona.md' });
    responderCom({ 'PUT /api/arquivo': put.rota });

    await aplicarOperacao(
      op({ tipo: 'editar_persona', agente: 'agentes/concierge', texto: 'Seja gentil.' }),
      ambiente(),
    );

    expect(put.corpos).toEqual([
      { agente: 'agentes/concierge', caminho: 'persona.md', conteudo: 'Seja gentil.' },
    ]);
  });

  it('writesAdvancedFile', async () => {
    const put = gravando({ salvo: 'rules/tom.md' });
    responderCom({ 'PUT /api/arquivo': put.rota });

    await aplicarOperacao(
      op({ tipo: 'editar_arquivo', caminho: 'rules/tom.md', conteudo: '' }),
      ambiente(),
    );

    expect(put.corpos).toEqual([{ agente: '', caminho: 'rules/tom.md', conteudo: '' }]);
  });

  it.each([
    {
      bruta: { tipo: 'preencher_textos', skill: 's', textos: { resumo: 'R', quandoUsar: 'Q' } },
      alvos: ['skill:s', 'skill:s:resumo', 'skill:s:quandoUsar'],
      rotas: {
        'GET /api/skills/s': () => json(textosLidos),
        'PUT /api/skills/s/textos': () => json({ versao: 'v8' }),
      },
    },
    {
      bruta: { tipo: 'trocar_fluxo', skill: 's', fluxo },
      alvos: ['skill:s', 'skill:s:fluxo'],
      rotas: {
        'GET /api/skills/s/fluxo': () => json({ fluxo, versao: 'f1', problemas: [] }),
        'PUT /api/skills/s/fluxo': () => json({ versao: 'f2', problemas: [] }),
      },
    },
    {
      bruta: { tipo: 'criar_skill', titulo: 'Nova skill' },
      alvos: ['skill:nova-skill'],
      rotas: { 'POST /api/skills': () => json({ nome: 'nova-skill' }) },
    },
    {
      bruta: { tipo: 'excluir_skill', skill: 'agendar-mudanca' },
      alvos: [],
      rotas: {
        'GET /api/skills/agendar-mudanca': () => json(textosLidos),
        'DELETE /api/skills/agendar-mudanca': () => new Response(null, { status: 204 }),
      },
    },
    {
      bruta: { tipo: 'habilitar_skill', condominio: 'c', skill: 's', habilitado: false },
      alvos: ['condominio:c', 'condominio:c:skill:s'],
      rotas: { 'PUT /api/admin/habilitacao': () => json(mapeamento) },
    },
    {
      bruta: { tipo: 'orientar', condominio: 'c', skill: 's', texto: 'T' },
      alvos: ['condominio:c', 'condominio:c:skill:s:orientacao'],
      rotas: { 'PUT /api/admin/orientacao': () => json(mapeamento) },
    },
    {
      bruta: { tipo: 'orientar', condominio: 'c', texto: 'T' },
      alvos: ['condominio:c', 'condominio:c:orientacao'],
      rotas: { 'PUT /api/admin/orientacao': () => json(mapeamento) },
    },
    {
      bruta: { tipo: 'ajustar', condominio: 'c', recurso: 'r', campos: { a: 1 } },
      alvos: ['condominio:c', 'condominio:c:ajustes:r'],
      rotas: { 'PUT /api/admin/ajustes': () => json(mapeamento) },
    },
    {
      bruta: { tipo: 'editar_persona', agente: 'a', texto: 'T' },
      alvos: ['persona', 'arquivo:a:persona.md'],
      rotas: { 'PUT /api/arquivo': () => json({ salvo: 'persona.md' }) },
    },
    {
      bruta: { tipo: 'editar_arquivo', agente: 'a', caminho: 'rules/x.md', conteudo: 'C' },
      alvos: ['arquivo:a:rules/x.md'],
      rotas: { 'PUT /api/arquivo': () => json({ salvo: 'rules/x.md' }) },
    },
  ])('returnsHighlightTargets for $bruta.tipo', async ({ bruta, alvos, rotas }) => {
    responderCom(rotas);

    const resultado = await aplicarOperacao(op(bruta), ambiente());

    expect(resultado).toMatchObject({ alvos });
  });

  it('highlightsOnlyTheFlowBlocksThatChanged', async () => {
    const antes = {
      blocos: [
        { id: 'b1', tipo: 'responder', texto: 'Oi' },
        { id: 'b2', tipo: 'responder', texto: 'Tchau' },
      ],
      informacoes: [],
    };
    const depois = {
      blocos: [
        { id: 'b1', tipo: 'responder', texto: 'Oi' },
        { id: 'b2', tipo: 'responder', texto: 'Até logo' },
        { id: 'b3', tipo: 'responder', texto: 'Novo' },
      ],
      informacoes: [],
    };
    responderCom({
      'GET /api/skills/s/fluxo': () => json({ fluxo: antes, versao: 'f1', problemas: [] }),
      'PUT /api/skills/s/fluxo': () => json({ versao: 'f2', problemas: [] }),
    });

    const resultado = await aplicarOperacao(
      op({ tipo: 'trocar_fluxo', skill: 's', fluxo: depois }),
      ambiente(),
    );

    expect(resultado).toMatchObject({
      alvos: ['skill:s', 'skill:s:fluxo', 'skill:s:bloco:b2', 'skill:s:bloco:b3'],
    });
  });
});

describe('podeAplicar', () => {
  const eu = (permissoes: string[], administradoraId: string | null) =>
    EuSchema.parse({ id: 'u', nome: 'U', papel: 'p', permissoes, administradoraId, dev: false });
  const textos = op({ tipo: 'preencher_textos', skill: 's', textos: {} });
  const orientar = op({ tipo: 'orientar', condominio: 'c', texto: 'T' });

  it.each([
    { quem: eu(['skills.editar_textos'], null), operacao: textos, esperado: true },
    { quem: eu(['skills.editar_textos'], 'adm1'), operacao: textos, esperado: false },
    { quem: eu(['skills.ler'], null), operacao: textos, esperado: false },
    { quem: eu(['condominios.habilitar'], 'adm1'), operacao: orientar, esperado: true },
    { quem: eu([], 'adm1'), operacao: orientar, esperado: false },
  ])('allowsApplyingOnlyWithTypePermission (#$#)', ({ quem, operacao, esperado }) => {
    expect(podeAplicar(quem, operacao)).toBe(esperado);
  });
});
