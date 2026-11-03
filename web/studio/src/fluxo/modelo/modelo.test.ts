import { describe, expect, it } from 'vitest';
import fixture from './agendar-mudanca.fixture.json';
import { novoBloco, resumoDoBloco, TIPOS_INSERIVEIS } from './catalogo';
import { atualizarBloco, indiceAoSoltar, inserirBloco, moverBloco, removerBloco } from './edicao';
import {
  type Bloco,
  BlocoSchema,
  type Fluxo,
  FluxoLidoSchema,
  InformacaoSchema,
  TIPOS_BLOCO,
  type TipoBloco,
} from './tipos';
import { validarFluxo } from './validar';

const fluxoDe = (...blocos: Bloco[]): Fluxo => ({ blocos, informacoes: [] });

function bloco(tipo: TipoBloco, id: string, campos: Record<string, unknown> = {}): Bloco {
  return BlocoSchema.parse({ ...novoBloco(tipo, id), ...campos });
}

const buscar = (id: string) => bloco('buscar_dados', id, { recurso: 'unidades.listar' });
const perguntar = (id: string) => bloco('perguntar', id, { campos: ['data'] });
const confirmar = (id: string) => bloco('confirmar', id, { texto: 'Ok?' });
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
    expect(lido.fluxo.blocos[2]).toMatchObject({
      tipo: 'perguntar',
      campos: ['data_mudanca', 'periodo_mudanca'],
    });
  });
});

// Tabela espelhada de internal/okf/fluxo_validacao_test.go (TestValidarFluxo): os dois lados andam juntos.
const MSG_CONFIRMAR = 'Coloque um Confirmar antes de gravar no sistema.';
const MSG_FIM = 'O Responder precisa ser o último bloco.';

function blocosDeTipos(...tipos: TipoBloco[]): Bloco[] {
  return tipos.map((tipo, i) => {
    const id = `b${i + 1}`;
    switch (tipo) {
      case 'buscar_dados':
      case 'gravar_no_sistema':
        return bloco(tipo, id, { recurso: 'x.y' });
      case 'pedir_escolha':
        return bloco(tipo, id, { param: 'p' });
      case 'perguntar':
        return bloco(tipo, id, { campos: ['c'] });
      case 'reaproveitar':
        return bloco(tipo, id, { campos: ['c'], confirmacao: 'resposta' });
      case 'formulario':
        return bloco(tipo, id, { de: 'd' });
      case 'confirmar':
      case 'montar_texto':
      case 'responder':
        return bloco(tipo, id, { texto: 't' });
      default:
        throw new Error(`tipo sem bloco de teste: ${tipo satisfies never}`);
    }
  });
}

const comCampos = (blocos: Bloco[], indice: number, campos: Record<string, unknown>): Bloco[] =>
  blocos.map((b, i) => (i === indice ? bloco(b.tipo, b.id, { ...b, ...campos }) : b));

const problema = (blocoId: string | undefined, codigo: string, mensagem: string) =>
  blocoId === undefined
    ? { codigo, nivel: 'erro', mensagem }
    : { blocoId, codigo, nivel: 'erro', mensagem };

const informacao = (campos: Record<string, unknown>): Fluxo['informacoes'][number] =>
  InformacaoSchema.parse({ nome: 'cor', ...campos });

describe('validarFluxo', () => {
  it.each([
    {
      caso: 'fluxo completo',
      fluxo: fluxoDe(
        ...blocosDeTipos(
          'buscar_dados',
          'pedir_escolha',
          'perguntar',
          'confirmar',
          'gravar_no_sistema',
          'responder',
        ),
      ),
      esperado: [],
    },
    {
      caso: 'gravar sem confirmar',
      fluxo: fluxoDe(...blocosDeTipos('buscar_dados', 'gravar_no_sistema', 'responder')),
      esperado: [problema('b2', 'GRAVAR_SEM_CONFIRMAR', MSG_CONFIRMAR)],
    },
    {
      caso: 'cada confirmar vale uma gravacao',
      fluxo: fluxoDe(
        ...blocosDeTipos('confirmar', 'gravar_no_sistema', 'gravar_no_sistema', 'responder'),
      ),
      esperado: [problema('b3', 'GRAVAR_SEM_CONFIRMAR', MSG_CONFIRMAR)],
    },
    {
      caso: 'um confirmar nao vale para duas gravacoes',
      fluxo: fluxoDe(
        ...blocosDeTipos(
          'confirmar',
          'confirmar',
          'gravar_no_sistema',
          'gravar_no_sistema',
          'responder',
        ),
      ),
      esperado: [problema('b4', 'GRAVAR_SEM_CONFIRMAR', MSG_CONFIRMAR)],
    },
    {
      caso: 'confirmar depois de gravar nao vale',
      fluxo: fluxoDe(...blocosDeTipos('gravar_no_sistema', 'confirmar', 'responder')),
      esperado: [problema('b1', 'GRAVAR_SEM_CONFIRMAR', MSG_CONFIRMAR)],
    },
    {
      caso: 'sem responder',
      fluxo: fluxoDe(...blocosDeTipos('buscar_dados')),
      esperado: [
        problema(undefined, 'SEM_RESPONDER', 'O fluxo precisa terminar com um Responder.'),
      ],
    },
    {
      caso: 'fluxo sem nenhum bloco',
      fluxo: fluxoDe(),
      esperado: [
        problema(undefined, 'SEM_RESPONDER', 'O fluxo precisa terminar com um Responder.'),
      ],
    },
    {
      caso: 'responder fora do fim',
      fluxo: fluxoDe(...blocosDeTipos('responder', 'buscar_dados')),
      esperado: [problema('b1', 'RESPONDER_FORA_DO_FIM', MSG_FIM)],
    },
    {
      caso: 'dois responders',
      fluxo: fluxoDe(...blocosDeTipos('responder', 'responder')),
      esperado: [problema('b1', 'RESPONDER_FORA_DO_FIM', MSG_FIM)],
    },
    {
      caso: 'so responder',
      fluxo: fluxoDe(...blocosDeTipos('responder')),
      esperado: [
        problema(undefined, 'FLUXO_VAZIO', 'Adicione ao menos um bloco antes do Responder.'),
      ],
    },
    {
      caso: 'recurso vazio',
      fluxo: fluxoDe(...comCampos(blocosDeTipos('buscar_dados', 'responder'), 0, { recurso: '' })),
      esperado: [problema('b1', 'BLOCO_INCOMPLETO', 'Preencha a ferramenta.')],
    },
    {
      caso: 'extra repete campo do bloco',
      fluxo: fluxoDe(
        ...comCampos(blocosDeTipos('buscar_dados', 'responder'), 0, {
          guardarEm: 'x',
          extra: { save_as: 'x', timeout: 3 },
        }),
      ),
      esperado: [
        problema('b1', 'EXTRA_REPETE_CAMPO', 'Este bloco tem um campo extra repetido: save_as.'),
      ],
    },
    {
      caso: 'extra da informacao repete campo',
      fluxo: {
        blocos: blocosDeTipos('buscar_dados', 'responder'),
        informacoes: [informacao({ tipo: 'string', extra: { type: 'x', help: 'ok' } })],
      },
      esperado: [
        problema(
          undefined,
          'EXTRA_REPETE_CAMPO',
          'A informação cor tem um campo extra repetido: type.',
        ),
      ],
    },
    {
      caso: 'extra repete campo de escolha',
      fluxo: fluxoDe(
        ...comCampos(blocosDeTipos('pedir_escolha', 'responder'), 0, {
          titulo: 'T',
          extra: { title: 't' },
        }),
      ),
      esperado: [
        problema('b1', 'EXTRA_REPETE_CAMPO', 'Este bloco tem um campo extra repetido: title.'),
      ],
    },
    {
      caso: 'opcoes simples com rotulo diferente',
      fluxo: {
        blocos: blocosDeTipos('buscar_dados', 'responder'),
        informacoes: [
          informacao({
            opcoesSimples: true,
            opcoes: [
              { valor: 'a', rotulo: 'a' },
              { valor: 'b', rotulo: 'B' },
            ],
          }),
        ],
      },
      esperado: [
        problema(
          undefined,
          'OPCAO_ROTULO_PERDIDO',
          'A informação cor tem opções simples com rótulo diferente do valor; o rótulo se perderia.',
        ),
      ],
    },
    {
      caso: 'opcoes simples iguais passam',
      fluxo: {
        blocos: blocosDeTipos('buscar_dados', 'responder'),
        informacoes: [informacao({ opcoesSimples: true, opcoes: [{ valor: 'a', rotulo: 'a' }] })],
      },
      esperado: [],
    },
  ])('$caso', ({ fluxo, esperado }) => {
    expect(validarFluxo(fluxo)).toEqual(esperado);
  });

  it('ignoraExtraQueNaoRepeteChave', () => {
    const blocos = comCampos(blocosDeTipos('buscar_dados', 'responder'), 0, {
      extra: { with: 'texto solto' },
    });
    const informacoes = [informacao({ extra: { options: [] } })];
    expect(validarFluxo({ blocos, informacoes })).toEqual([]);
  });

  it('repeteExtraDeWithQuandoHaCampos', () => {
    const blocos = comCampos(blocosDeTipos('buscar_dados', 'responder'), 0, {
      com: [{ campo: 'a', valor: 1 }],
      extra: { with: 'x' },
    });
    expect(validarFluxo(fluxoDe(...blocos)).map((p) => p.codigo)).toEqual(['EXTRA_REPETE_CAMPO']);
  });

  it('ordenaProblemasComoOServidor', () => {
    const blocos = [
      bloco('gravar_no_sistema', 'b1', { recurso: '', guardarEm: 'x', extra: { save_as: 1 } }),
      bloco('responder', 'b2', { texto: 't' }),
      bloco('buscar_dados', 'b3', { recurso: '' }),
    ];
    const informacoes = [
      informacao({ opcoesSimples: true, opcoes: [{ valor: 'a', rotulo: 'A' }] }),
    ];
    expect(
      validarFluxo({ blocos, informacoes }).map((p) => `${p.blocoId ?? '-'}:${p.codigo}`),
    ).toEqual([
      'b1:BLOCO_INCOMPLETO',
      'b1:EXTRA_REPETE_CAMPO',
      'b3:BLOCO_INCOMPLETO',
      'b1:GRAVAR_SEM_CONFIRMAR',
      'b2:RESPONDER_FORA_DO_FIM',
      '-:OPCAO_ROTULO_PERDIDO',
    ]);
  });

  it('exigeOndeGuardarARespostaDoReaproveitar', () => {
    const blocos = comCampos(blocosDeTipos('reaproveitar', 'responder'), 0, { confirmacao: '' });
    expect(validarFluxo(fluxoDe(...blocos))).toEqual([
      problema('b1', 'BLOCO_INCOMPLETO', 'Preencha onde guardar a resposta.'),
    ]);
  });

  it('semResponderVemDepoisDosProblemasDeBloco', () => {
    const blocos = comCampos(blocosDeTipos('buscar_dados'), 0, { recurso: '' });
    expect(validarFluxo(fluxoDe(...blocos)).map((p) => p.codigo)).toEqual([
      'BLOCO_INCOMPLETO',
      'SEM_RESPONDER',
    ]);
  });

  it.each([
    'buscar_dados',
    'gravar_no_sistema',
    'pedir_escolha',
    'perguntar',
    'formulario',
    'confirmar',
    'montar_texto',
    'reaproveitar',
    'responder',
  ] as const)('exigeCampoDe %s', (tipo) => {
    expect(validarFluxo(fluxoDe(bloco(tipo, 'b1')))).toContainEqual(
      expect.objectContaining({ blocoId: 'b1', codigo: 'BLOCO_INCOMPLETO' }),
    );
  });

  it('aceitaOFluxoReal', () => {
    expect(validarFluxo(FluxoLidoSchema.parse(fixture).fluxo)).toEqual([]);
  });

  it('aceitaFluxoComListasNulasDoGo', () => {
    const lido = FluxoLidoSchema.parse({
      fluxo: { blocos: null, informacoes: null },
      versao: 'v',
      problemas: null,
    });
    expect(lido).toMatchObject({ fluxo: { blocos: [], informacoes: [] }, problemas: [] });
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
    {
      b: bloco('buscar_dados', 'a', { recurso: 'unidades.listar' }),
      esperado: 'TITULO(unidades.listar)',
    },
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
