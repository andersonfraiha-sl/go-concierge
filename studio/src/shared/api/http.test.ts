import { describe, expect, it, vi } from 'vitest';
import { z } from 'zod';
import { json, responderCom } from '@/test/fetchFalso';
import { apiEnviar, apiGet, ApiError, CHAVE_PAPEL_DEV } from './http';

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

  it('enviaPapelDeDesenvolvimentoQuandoGravado', async () => {
    localStorage.setItem(CHAVE_PAPEL_DEV, 'gestor');
    let papel: string | null = null;
    responderCom({
      'GET /api/item': (req) => {
        papel = req.headers.get('X-Studio-Papel');
        return json({ nome: 'a' });
      },
    });
    await apiGet('/api/item', Item);
    expect(papel).toBe('gestor');
  });

  it('omitePapelQuandoNaoGravado', async () => {
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
    await expect(apiEnviar('PUT', '/api/item', { nome: 'b' }, Item)).resolves.toEqual({ nome: 'b' });
    expect(recebido).toEqual({ nome: 'b' });
  });
});
