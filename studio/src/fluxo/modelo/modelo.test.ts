import { describe, expect, it } from 'vitest';
import fixture from './agendar-mudanca.fixture.json';
import { novoBloco, resumoDoBloco, TIPOS_INSERIVEIS } from './catalogo';
import { atualizarBloco, indiceAoSoltar, inserirBloco, moverBloco, removerBloco } from './edicao';
import { BlocoSchema, FluxoLidoSchema, TIPOS_BLOCO, type Bloco, type Fluxo, type TipoBloco } from './tipos';
import { validarFluxo } from './validar';

const fluxoDe = (...blocos: Bloco[]): Fluxo => ({ blocos, informacoes: [] });

function bloco(tipo: TipoBloco, id: string, campos: Record<string, unknown> = {}): Bloco {
  return BlocoSchema.parse({ ...novoBloco(tipo, id), ...campos });
}

const buscar = (id: string) => bloco('buscar_dados', id, { recurso: 'unidades.listar' });
const escolha = (id: string) => bloco('pedir_escolha', id, { param: 'unidade' });
const perguntar = (id: string) => bloco('perguntar', id, { campos: ['data'] });
const confirmar = (id: string) => bloco('confirmar', id, { texto: 'Ok?' });
const gravar = (id: string) => bloco('gravar_no_sistema', id, { recurso: 'mudancas.agendar' });
const responder = (id: string) => bloco('responder', id, { texto: 'Pronto' });

describe('FluxoLidoSchema', () => {
  it('leFixtureDaSkillAgendarMudanca', () => {
    const lido = FluxoLidoSchema.parse(fixture);
    expect(lido.fluxo.blocos).toHaveLength(6);
    expect(lido.fluxo.blocos.at(-1)?.tipo).toBe('responder');
    expect(lido.fluxo.informacoes[1]?.opcoesSimples).toBe(false);
  });

  it('recusaTipoDeBlocoDesconhecido', () => {
    const invalido = { ...fixture.fluxo, blocos: [{ id: 'b1', tipo: 'voar' }] };
    expect(FluxoLidoSchema.safeParse({ ...fixture, fluxo: invalido }).success).toBe(false);
  });

  it('preencheDefaultsDeCampoQueVeioComOmitempty', () => {
    const lido = FluxoLidoSchema.parse(fixture);
    expect(lido.fluxo.blocos[0]).toMatchObject({ tipo: 'buscar_dados', com: [] });
    expect(lido.fluxo.blocos[2]).toMatchObject({ tipo: 'perguntar', campos: ['data_mudanca', 'periodo_mudanca'] });
  });
});

describe('validarFluxo', () => {
  const resumo = (f: Fluxo) => validarFluxo(f).map(({ codigo, blocoId }) => ({ codigo, blocoId }));

  it.each([
    {
      caso: 'a) fluxo completo',
      fluxo: fluxoDe(buscar('b1'), escolha('b2'), perguntar('b3'), confirmar('b4'), gravar('b5'), responder('b6')),
      esperado: [],
    },
    {
      caso: 'b) gravar sem confirmar',
      fluxo: fluxoDe(buscar('b1'), gravar('b2'), responder('b3')),
      esperado: [{ codigo: 'GRAVAR_SEM_CONFIRMAR', blocoId: 'b2' }],
    },
    {
      caso: 'c) um confirmar vale para uma gravacao',
      fluxo: fluxoDe(confirmar('b1'), gravar('b2'), gravar('b3'), responder('b4')),
      esperado: [{ codigo: 'GRAVAR_SEM_CONFIRMAR', blocoId: 'b3' }],
    },
    {
      caso: 'd) confirmar depois do gravar nao vale',
      fluxo: fluxoDe(gravar('b1'), confirmar('b2'), responder('b3')),
      esperado: [{ codigo: 'GRAVAR_SEM_CONFIRMAR', blocoId: 'b1' }],
    },
    {
      caso: 'e) sem responder',
      fluxo: fluxoDe(buscar('b1')),
      esperado: [{ codigo: 'SEM_RESPONDER', blocoId: undefined }],
    },
    {
      caso: 'f) responder fora do fim',
      fluxo: fluxoDe(responder('b1'), buscar('b2')),
      esperado: [{ codigo: 'RESPONDER_FORA_DO_FIM', blocoId: 'b1' }],
    },
    {
      caso: 'g) dois responders no fim',
      fluxo: fluxoDe(buscar('b1'), responder('b2'), responder('b3')),
      esperado: [{ codigo: 'RESPONDER_FORA_DO_FIM', blocoId: 'b2' }],
    },
    {
      caso: 'h) so responder',
      fluxo: fluxoDe(responder('b1')),
      esperado: [{ codigo: 'FLUXO_VAZIO', blocoId: undefined }],
    },
    {
      caso: 'i) buscar sem ferramenta',
      fluxo: fluxoDe(bloco('buscar_dados', 'b1'), responder('b2')),
      esperado: [{ codigo: 'BLOCO_INCOMPLETO', blocoId: 'b1' }],
    },
  ])('$caso', ({ fluxo, esperado }) => {
    expect(resumo(fluxo)).toEqual(esperado);
  });

  it('devolveNivelEMensagemExatos', () => {
    expect(validarFluxo(fluxoDe(bloco('buscar_dados', 'b1'), responder('b2')))).toEqual([
      { blocoId: 'b1', codigo: 'BLOCO_INCOMPLETO', nivel: 'erro', mensagem: 'Preencha a ferramenta.' },
    ]);
    expect(validarFluxo(fluxoDe(buscar('b1'), gravar('b2'), responder('b3')))).toEqual([
      {
        blocoId: 'b2',
        codigo: 'GRAVAR_SEM_CONFIRMAR',
        nivel: 'erro',
        mensagem: 'Coloque um Confirmar antes de gravar no sistema.',
      },
    ]);
    expect(validarFluxo(fluxoDe(buscar('b1')))).toEqual([
      { codigo: 'SEM_RESPONDER', nivel: 'erro', mensagem: 'O fluxo precisa terminar com um Responder.' },
    ]);
    expect(validarFluxo(fluxoDe(responder('b1'), buscar('b2')))[0]).toEqual({
      blocoId: 'b1',
      codigo: 'RESPONDER_FORA_DO_FIM',
      nivel: 'erro',
      mensagem: 'O Responder precisa ser o último bloco.',
    });
    expect(validarFluxo(fluxoDe(responder('b1')))).toEqual([
      { codigo: 'FLUXO_VAZIO', nivel: 'erro', mensagem: 'Adicione ao menos um bloco antes do Responder.' },
    ]);
  });

  it.each([
    { tipo: 'pedir_escolha', rotulo: 'onde guardar a escolha' },
    { tipo: 'perguntar', rotulo: 'as informações perguntadas' },
    { tipo: 'formulario', rotulo: 'de onde vêm os campos' },
    { tipo: 'confirmar', rotulo: 'a pergunta de confirmação' },
    { tipo: 'montar_texto', rotulo: 'a instrução' },
    { tipo: 'reaproveitar', rotulo: 'as informações reaproveitadas' },
    { tipo: 'gravar_no_sistema', rotulo: 'a ferramenta' },
  ] as const)('pedePreenchimentoDe $tipo', ({ tipo, rotulo }) => {
    const [problema] = validarFluxo(fluxoDe(confirmar('b0'), bloco(tipo, 'b1'), responder('b2')));
    expect(problema).toMatchObject({ blocoId: 'b1', codigo: 'BLOCO_INCOMPLETO', mensagem: `Preencha ${rotulo}.` });
  });

  it('pedeAResposta', () => {
    const [problema] = validarFluxo(fluxoDe(buscar('b1'), bloco('responder', 'b2')));
    expect(problema).toMatchObject({ blocoId: 'b2', mensagem: 'Preencha a resposta.' });
  });

  it('aceitaOFluxoReal', () => {
    expect(validarFluxo(FluxoLidoSchema.parse(fixture).fluxo)).toEqual([]);
  });
});

describe('edicao', () => {
  const base = () => fluxoDe(buscar('b1'), perguntar('b2'), confirmar('b3'), responder('b4'));
  const ids = (f: Fluxo) => f.blocos.map((b) => b.id);

  it('insereNoInicio', () => {
    expect(ids(inserirBloco(base(), 0, buscar('n')))).toEqual(['n', 'b1', 'b2', 'b3', 'b4']);
  });

  it('insereNoMeio', () => {
    expect(ids(inserirBloco(base(), 2, buscar('n')))).toEqual(['b1', 'b2', 'n', 'b3', 'b4']);
  });

  it('mantemOResponderNoFimAoInserirDepoisDele', () => {
    expect(ids(inserirBloco(base(), 99, buscar('n')))).toEqual(['b1', 'b2', 'b3', 'n', 'b4']);
  });

  it('naoMutaOFluxoDeEntrada', () => {
    const f = base();
    inserirBloco(f, 0, buscar('n'));
    moverBloco(f, 0, 2);
    removerBloco(f, 'b1');
    expect(ids(f)).toEqual(['b1', 'b2', 'b3', 'b4']);
  });

  it('moveParaOFimSemPassarDoResponder', () => {
    expect(ids(moverBloco(base(), 0, 99))).toEqual(['b2', 'b3', 'b1', 'b4']);
  });

  it('moveParaCimaELimitaEmZero', () => {
    expect(ids(moverBloco(base(), 2, -5))).toEqual(['b3', 'b1', 'b2', 'b4']);
  });

  it('naoMoveOResponder', () => {
    const f = base();
    expect(moverBloco(f, 3, 0)).toBe(f);
  });

  it('removeBlocoPorId', () => {
    expect(ids(removerBloco(base(), 'b2'))).toEqual(['b1', 'b3', 'b4']);
  });

  it('naoRemoveOResponder', () => {
    const f = base();
    expect(removerBloco(f, 'b4')).toBe(f);
  });

  it('atualizaBlocoPorId', () => {
    const novo = bloco('confirmar', 'b3', { texto: 'Outro' });
    expect(atualizarBloco(base(), novo).blocos[2]).toEqual(novo);
  });

  it.each([
    { y: 0, total: 6, esperado: 0 },
    { y: 125, total: 6, esperado: 1 },
    { y: 10000, total: 6, esperado: 4 },
    { y: -50, total: 6, esperado: 0 },
    { y: 500, total: 1, esperado: 0 },
  ])('indiceAoSoltar(y=$y, total=$total) = $esperado', ({ y, total, esperado }) => {
    expect(indiceAoSoltar(y, total)).toBe(esperado);
  });
});

describe('catalogo de blocos', () => {
  it('omiteOResponderDosInseriveis', () => {
    expect(TIPOS_INSERIVEIS).toEqual(TIPOS_BLOCO.filter((t) => t !== 'responder'));
  });

  it('criaBlocoVazioComOsDefaults', () => {
    expect(novoBloco('perguntar', 'x')).toMatchObject({ id: 'x', tipo: 'perguntar', campos: [] });
  });

  const titulo = (nome: string) => `TITULO(${nome})`;

  it.each([
    { b: bloco('buscar_dados', 'a', { recurso: 'unidades.listar' }), esperado: 'TITULO(unidades.listar)' },
    { b: bloco('gravar_no_sistema', 'a', { recurso: 'x.y' }), esperado: 'TITULO(x.y)' },
    { b: bloco('pedir_escolha', 'a', { pergunta: 'Qual unidade?' }), esperado: 'Qual unidade?' },
    { b: bloco('perguntar', 'a', { campos: ['data', 'periodo'] }), esperado: 'data, periodo' },
    { b: bloco('formulario', 'a', { de: 'campos' }), esperado: 'Campos de campos' },
    { b: bloco('reaproveitar', 'a', { campos: ['unidade'] }), esperado: 'unidade' },
    { b: bloco('confirmar', 'a', { texto: 'Linha 1\n  linha   2' }), esperado: 'Linha 1 linha 2' },
    { b: bloco('montar_texto', 'a'), esperado: 'Clique para configurar' },
    { b: bloco('buscar_dados', 'a'), esperado: 'Clique para configurar' },
  ])('resumeBloco $b.tipo', ({ b, esperado }) => {
    expect(resumoDoBloco(b, titulo)).toBe(esperado);
  });

  it('cortaEm60CaracteresComReticencias', () => {
    const resumo = resumoDoBloco(bloco('responder', 'a', { texto: 'x'.repeat(100) }), titulo);
    expect(resumo).toBe(`${'x'.repeat(60)}…`);
  });

  it('naoCortaTextoDe60Caracteres', () => {
    const texto = 'y'.repeat(60);
    expect(resumoDoBloco(bloco('responder', 'a', { texto }), titulo)).toBe(texto);
  });
});
