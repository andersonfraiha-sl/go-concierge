import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import {
  AgenteSchema,
  CAMINHO_PERSONA,
  caminhoDeDocumento,
  caminhoDeRegra,
  excluirArquivo,
  FerramentasSchema,
  lerModeloDeArquivo,
  salvarArquivo,
  vincularFerramenta,
} from './contratos';

const agente = {
  agente: {
    caminho: 'agentes/concierge/agentes/agendar-mudanca',
    nome: 'agendar-mudanca',
    descricao: 'Agenda mudança',
    persona: '',
    tipo: 'subagente',
    tipo_declarado: true,
    arquivos: {
      rules: [{ caminho: 'rules/horario.md', bytes: 120, resumo: 'Horário' }],
    },
    herdados: { rules: [{ caminho: 'rules/tom.md', bytes: 10, herdado_de: 'concierge' }] },
    problemas: [{ nivel: 'aviso', onde: 'agent.yaml', texto: 'Sem modelo', solucao: 'Defina um' }],
    subagentes: [],
  },
  vinculos: [],
  recursos: [
    { nome: 'unidades.listar', origem: 'caso.yaml', ok: true, conferido: true },
    { nome: 'x.y', origem: 'agent.yaml', ok: false, problema: 'não existe', conferido: true },
  ],
};

describe('AgenteSchema', () => {
  it('leARespostaDeApiAgente', () => {
    const lido = AgenteSchema.parse(agente);
    expect(lido.agente.arquivos.rules).toHaveLength(1);
    expect(lido.agente.herdados.rules?.[0]?.herdado_de).toBe('concierge');
    expect(lido.recursos).toHaveLength(2);
  });

  it('toleraListasNulasDoGo', () => {
    const lido = AgenteSchema.parse({
      agente: { ...agente.agente, arquivos: null, herdados: null, problemas: null },
      recursos: null,
    });
    expect(lido).toMatchObject({
      agente: { arquivos: {}, herdados: {}, problemas: [] },
      recursos: [],
    });
  });
});

describe('FerramentasSchema', () => {
  it('leOInstantaneoDeRecursos', () => {
    const lido = FerramentasSchema.parse({
      account: 'superlogica',
      origem: 'instantaneo',
      conferido: true,
      recursos: [
        {
          name: 'mudancas.agendar',
          titulo: 'Agendar mudança',
          kind: 'api',
          visibility: 'private',
          write: true,
          description: 'Agenda',
          params: [{ name: 'data', type: 'string', required: true }],
          output: 'protocolo',
          saida: [{ name: 'protocolo', path: 'data.protocolo' }],
          amostra: { protocolo: 'ABC' },
        },
        {
          name: 'saude',
          kind: 'api',
          visibility: 'public',
          write: false,
          description: '',
          params: null,
        },
      ],
    });
    expect(lido.recursos[0]?.write).toBe(true);
    expect(lido.recursos[1]?.params).toEqual([]);
  });
});

describe('caminhos', () => {
  it('montaCaminhosDeRegraEDocumento', () => {
    expect(caminhoDeRegra('horario.md')).toBe('rules/horario.md');
    expect(caminhoDeDocumento('manual.md')).toBe('knowledge/manual.md');
    expect(CAMINHO_PERSONA).toBe('persona.md');
  });
});

describe('chamadas', () => {
  it('salvaArquivoComPut', async () => {
    let corpo: unknown = null;
    responderCom({
      'PUT /api/arquivo': async (req) => {
        corpo = await req.json();
        return json({ salvo: 'rules/a.md' });
      },
    });
    await expect(
      salvarArquivo({ agente: 'ag', caminho: 'rules/a.md', conteudo: 'oi' }),
    ).resolves.toBeUndefined();
    expect(corpo).toEqual({ agente: 'ag', caminho: 'rules/a.md', conteudo: 'oi' });
  });

  it('excluiArquivoPelaQuery', async () => {
    let url = '';
    responderCom({
      'DELETE /api/arquivo': (req) => {
        url = req.url;
        return json({ excluido: 'rules/a b.md' });
      },
    });
    await excluirArquivo('ag/x', 'rules/a b.md');
    expect(new URL(url).searchParams.get('agente')).toBe('ag/x');
    expect(new URL(url).searchParams.get('caminho')).toBe('rules/a b.md');
  });

  it('leOModeloDeArquivo', async () => {
    responderCom({
      'GET /api/arquivo/modelo': () => json({ caminho: 'rules/a.md', conteudo: '# Regra' }),
    });
    await expect(lerModeloDeArquivo('rules/a.md')).resolves.toBe('# Regra');
  });

  it('vinculaFerramenta', async () => {
    let corpo: unknown = null;
    responderCom({
      'POST /api/agente/recursos': async (req) => {
        corpo = await req.json();
        return json({ recurso: 'x', ligado: true, mudancas: ['nada a fazer'] });
      },
    });
    await expect(
      vincularFerramenta({ agente: 'ag', nome: 'x', ligar: true }),
    ).resolves.toBeUndefined();
    expect(corpo).toEqual({ agente: 'ag', nome: 'x', ligar: true });
  });
});
