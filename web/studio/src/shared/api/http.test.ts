import { describe, expect, it, vi } from 'vitest';
import { z } from 'zod';
import { json, responderCom } from '@/test/fetchFalso';
import { ApiError, apiEnviar, apiEnviarBinario, apiGet, CHAVE_USUARIO_DEV } from './http';

const Item = z.object({ nome: z.string() });

async function capturarErro(promessa: Promise<unknown>): Promise<ApiError> {
  try {
    await promessa;
  } catch (erro) {
    if (erro instanceof ApiError) return erro;
    throw erro;
  }
  throw new Error('esperava ApiError, mas a chamada resolveu');
}

describe('apiGet', () => {
  it('devolveDadoParseadoNo200', async () => {
    responderCom({ 'GET /api/item': () => json({ nome: 'a' }) });
    await expect(apiGet('/api/item', Item)).resolves.toEqual({ nome: 'a' });
  });

  it('parseiaNuloNo204', async () => {
    responderCom({ 'GET /api/item': () => new Response(null, { status: 204 }) });
    await expect(apiGet('/api/item', z.null())).resolves.toBeNull();
  });

  it.each([
    {
      caso: 'formatoNovo',
      status: 422,
      corpo: { codigo: 'FLUXO_INVALIDO', mensagem: 'x' },
      codigo: 'FLUXO_INVALIDO',
      mensagem: 'x',
    },
    {
      caso: 'formatoAntigo',
      status: 400,
      corpo: { error: 'y' },
      codigo: 'HTTP_400',
      mensagem: 'y',
    },
    {
      caso: 'codigoMalformadoPreservaMensagem',
      status: 422,
      corpo: { codigo: 42, mensagem: 'z' },
      codigo: 'HTTP_422',
      mensagem: 'z',
    },
    {
      caso: 'corpoVazio',
      status: 500,
      corpo: {},
      codigo: 'HTTP_500',
      mensagem: 'Não deu para falar com o servidor.',
    },
  ])('lancaApiErrorNoErroHttp: $caso', async ({ status, corpo, codigo, mensagem }) => {
    responderCom({ 'GET /api/item': () => json(corpo, status) });
    const erro = await capturarErro(apiGet('/api/item', Item));
    expect(erro).toMatchObject({ status, codigo, message: mensagem, corpo });
  });

  it('lancaSemConexaoQuandoFetchRejeita', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('rede')));
    const erro = await capturarErro(apiGet('/api/item', Item));
    expect(erro).toMatchObject({ status: 0, codigo: 'SEM_CONEXAO' });
  });

  it('lancaRespostaInvalidaQuandoCorpoForaDoSchema', async () => {
    const erroNoConsole = vi.spyOn(console, 'error').mockImplementation(() => {});
    responderCom({ 'GET /api/item': () => json({ nome: 1 }) });
    const erro = await capturarErro(apiGet('/api/item', Item));
    expect(erro).toMatchObject({ status: 200, codigo: 'RESPOSTA_INVALIDA' });
    expect(erroNoConsole).toHaveBeenCalled();
  });

  it('sendsDevUserHeaderOnEveryCall', async () => {
    localStorage.setItem(CHAVE_USUARIO_DEV, 'gestor-exemplo');
    const recebidos: (string | null)[] = [];
    const registrar = (req: Request) => {
      recebidos.push(req.headers.get('X-Dev-Usuario'));
      return json({ nome: 'a' });
    };
    responderCom({
      'GET /api/item': registrar,
      'PUT /api/item': registrar,
      'DELETE /api/item': registrar,
    });
    await apiGet('/api/item', Item);
    await apiEnviar('PUT', '/api/item', { nome: 'a' }, Item);
    await apiEnviar('DELETE', '/api/item', undefined, Item);
    expect(recebidos).toEqual(['gestor-exemplo', 'gestor-exemplo', 'gestor-exemplo']);
  });

  it('omitsDevUserHeaderWhenNotStored', async () => {
    let usuario: string | null = 'sentinela';
    responderCom({
      'GET /api/item': (req) => {
        usuario = req.headers.get('X-Dev-Usuario');
        return json({ nome: 'a' });
      },
    });
    await apiGet('/api/item', Item);
    expect(usuario).toBeNull();
  });

  it('stopsSendingStudioRoleHeader', async () => {
    localStorage.setItem('studio.papelDev', 'construtor');
    let papel: string | null = 'sentinela';
    responderCom({
      'GET /api/item': (req) => {
        papel = req.headers.get('X-Studio-Papel');
        return json({ nome: 'a' });
      },
    });
    await apiGet('/api/item', Item);
    expect(papel).toBeNull();
  });
});

describe('apiEnviar', () => {
  it('enviaCorpoJsonNoPut', async () => {
    let recebido: unknown = null;
    responderCom({
      'PUT /api/item': async (req) => {
        recebido = await req.json();
        return json({ nome: 'b' });
      },
    });
    await expect(apiEnviar('PUT', '/api/item', { nome: 'b' }, Item)).resolves.toEqual({
      nome: 'b',
    });
    expect(recebido).toEqual({ nome: 'b' });
  });
});

describe('apiEnviar com cancelamento', () => {
  function esperarAborto(_: RequestInfo | URL, init?: RequestInit): Promise<Response> {
    return new Promise((_resolver, rejeitar) => {
      init?.signal?.addEventListener('abort', () => rejeitar(init.signal?.reason));
    });
  }

  it('keepsDeadlineWhenCallerPassesSignal', async () => {
    vi.stubGlobal('fetch', vi.fn(esperarAborto));
    const quemChama = new AbortController();

    const erro = await capturarErro(
      apiEnviar('POST', '/api/item', {}, Item, { sinal: quemChama.signal, timeoutMs: 5 }),
    );

    expect([erro.codigo, erro.message]).toEqual([
      'TEMPO_ESGOTADO',
      'O servidor demorou demais para responder.',
    ]);
  });

  it('cancelsWithCallerSignal', async () => {
    vi.stubGlobal('fetch', vi.fn(esperarAborto));
    const quemChama = new AbortController();

    const pedido = capturarErro(
      apiEnviar('POST', '/api/item', {}, Item, { sinal: quemChama.signal }),
    );
    quemChama.abort();

    expect((await pedido).codigo).toBe('CANCELADO');
  });
});

describe('apiEnviarBinario', () => {
  const zip = new Blob(['zip']);

  it('enviaCorpoComTipoEUsuarioDev', async () => {
    localStorage.setItem(CHAVE_USUARIO_DEV, 'construtor-superlogica');
    const falso = responderCom({ 'POST /api/zip': () => json({ nome: 'a' }) });
    await expect(
      apiEnviarBinario('POST', '/api/zip', zip, 'application/zip', Item),
    ).resolves.toEqual({ nome: 'a' });
    const req = new Request('http://localhost/api/zip', falso.mock.calls[0]?.[1]);
    expect(req.headers.get('Content-Type')).toBe('application/zip');
    expect(req.headers.get('X-Dev-Usuario')).toBe('construtor-superlogica');
  });

  it('traduzFalhaDeRedeEmSemConexao', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('x')));
    const erro = await capturarErro(
      apiEnviarBinario('POST', '/api/zip', zip, 'application/zip', Item),
    );
    expect(erro.codigo).toBe('SEM_CONEXAO');
  });

  it('mantemCodigoEMensagemDoServidor', async () => {
    responderCom({
      'POST /api/zip': () => json({ codigo: 'ZIP_INVALIDO', mensagem: 'Zip corrompido.' }, 400),
    });
    const erro = await capturarErro(
      apiEnviarBinario('POST', '/api/zip', zip, 'application/zip', Item),
    );
    expect([erro.status, erro.codigo, erro.message]).toEqual([
      400,
      'ZIP_INVALIDO',
      'Zip corrompido.',
    ]);
  });
});
